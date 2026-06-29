package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// newScanTestDaemon wires a Daemon with the given client baseURL so
// uploadFinalImageAttachments hits a controllable httptest server.
func newScanTestDaemon(t *testing.T, baseURL string) *Daemon {
	t.Helper()
	d := &Daemon{
		client: NewClient(baseURL),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return d
}

func writeImage(t *testing.T, dir, name string, size int) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return full
}

func TestUploadFinalImageAttachments_UploadsLocalImagesAndSkipsRemote(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "att-x",
			"task_id":     "task-1",
			"kind":        "image",
			"mime_type":   "image/png",
			"byte_size":   42,
			"storage_key": "tasks/task-1/att-x.png",
		})
	}))
	defer srv.Close()

	workDir := t.TempDir()
	writeImage(t, workDir, "chart.png", 100)
	writeImage(t, workDir, "nested/diagram.jpg", 200)

	comment := strings.Join([]string{
		"# 报告",
		"![alt](chart.png)",
		"![](nested/diagram.jpg)",
		"![duplicate](chart.png)",
		"![remote](https://example.com/x.png)",
		"![non-image](report.txt)",
	}, "\n")

	d := newScanTestDaemon(t, srv.URL)
	got := d.uploadFinalImageAttachments(context.Background(), "task-1", workDir, comment, d.logger)

	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected 2 uploads (local png + jpg, dedup, skip remote/non-image), got %d", n)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 returned attachments, got %d", len(got))
	}
}

func TestUploadFinalImageAttachments_RefusesPathTraversal(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	root := t.TempDir()
	workDir := filepath.Join(root, "agent")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create the file the malicious ref tries to reach: outside workDir.
	writeImage(t, root, "secret.png", 16)

	comment := "![](../secret.png)"

	d := newScanTestDaemon(t, srv.URL)
	got := d.uploadFinalImageAttachments(context.Background(), "task-1", workDir, comment, d.logger)
	if len(got) != 0 {
		t.Fatalf("expected no uploads, got %d", len(got))
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("expected zero server calls (path-traversal blocked), got %d", n)
	}
}

func TestUploadFinalImageAttachments_SkipsOversizedFiles(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()

	workDir := t.TempDir()
	// 1 byte over the cap.
	writeImage(t, workDir, "huge.png", int(MaxTaskAttachmentSize+1))

	d := newScanTestDaemon(t, srv.URL)
	got := d.uploadFinalImageAttachments(context.Background(), "task-1", workDir, "![](huge.png)", d.logger)
	if len(got) != 0 {
		t.Fatalf("expected no uploads, got %d", len(got))
	}
	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("expected zero server calls (oversized skipped), got %d", n)
	}
}

func TestUploadFinalImageAttachments_NoCommentNoOp(t *testing.T) {
	d := newScanTestDaemon(t, "http://unused.example")
	if got := d.uploadFinalImageAttachments(context.Background(), "task-1", t.TempDir(), "", d.logger); got != nil {
		t.Errorf("expected nil for empty comment, got %+v", got)
	}
	if got := d.uploadFinalImageAttachments(context.Background(), "task-1", "", "![](x.png)", d.logger); got != nil {
		t.Errorf("expected nil for empty workDir, got %+v", got)
	}
}
