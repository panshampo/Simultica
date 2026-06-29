package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadTaskAttachment_TooLargeRejectedClientSide(t *testing.T) {
	c := NewClient("http://unused.example")
	data := make([]byte, MaxTaskAttachmentSize+1)
	_, err := c.UploadTaskAttachment(context.Background(), "task-1", "big.png", "image/png", data, "")
	if !errors.Is(err, ErrTaskAttachmentTooLarge) {
		t.Fatalf("expected ErrTaskAttachmentTooLarge, got %v", err)
	}
}

func TestUploadTaskAttachment_MultipartShape(t *testing.T) {
	var (
		gotPath      string
		gotMessageID string
		gotFilename  string
		gotMime      string
		gotBytes     []byte
		gotAuth      string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("expected multipart/form-data, got %q (err %v)", mediaType, err)
			http.Error(w, "bad content-type", http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read multipart part: %v", err)
			}
			switch part.FormName() {
			case "message_id":
				buf, _ := io.ReadAll(part)
				gotMessageID = string(buf)
			case "file":
				gotFilename = part.FileName()
				gotMime = part.Header.Get("Content-Type")
				gotBytes, _ = io.ReadAll(part)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":          "att-1",
			"task_id":     "task-1",
			"message_id":  "",
			"kind":        "image",
			"mime_type":   "image/png",
			"byte_size":   len(gotBytes),
			"storage_key": "tasks/task-1/att-1.png",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.SetToken("tok")
	resp, err := c.UploadTaskAttachment(context.Background(), "task-1", "chart.png", "image/png", []byte("PNG-DATA"), "msg-1")
	if err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}

	if gotPath != "/api/daemon/tasks/task-1/attachments" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotMessageID != "msg-1" {
		t.Errorf("message_id = %q", gotMessageID)
	}
	if gotFilename != "chart.png" {
		t.Errorf("filename = %q", gotFilename)
	}
	if gotMime != "image/png" {
		t.Errorf("mime = %q", gotMime)
	}
	if string(gotBytes) != "PNG-DATA" {
		t.Errorf("bytes = %q", string(gotBytes))
	}
	if resp.ID != "att-1" || resp.Kind != "image" {
		t.Errorf("unexpected resp: %+v", resp)
	}
}

func TestUploadTaskAttachment_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "kaboom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.UploadTaskAttachment(context.Background(), "task-1", "f.png", "image/png", []byte("x"), "")
	if err == nil {
		t.Fatalf("expected error from 500 response")
	}
	var rerr *requestError
	if !errors.As(err, &rerr) || rerr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected requestError 500, got %v", err)
	}
	if !strings.Contains(rerr.Body, "kaboom") {
		t.Errorf("expected body to include server error text, got %q", rerr.Body)
	}
}
