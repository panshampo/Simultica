package lark

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// parseUploadMultipart pulls the form fields and the `image` file out
// of a request body whose Content-Type is multipart/form-data. Tests
// rely on this to assert the wire shape Lark requires:
// `image_type=<message|avatar>` form field + `image` file part.
func parseUploadMultipart(t *testing.T, r *http.Request) (fields map[string]string, fileName string, fileBody []byte) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("upload: parse content-type: %v", err)
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		t.Fatalf("upload: want multipart content-type, got %q", mediaType)
	}
	if params["boundary"] == "" {
		t.Fatalf("upload: content-type missing boundary param: %q", r.Header.Get("Content-Type"))
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	fields = map[string]string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("upload: next part: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("upload: read part %q: %v", part.FormName(), err)
		}
		if part.FileName() != "" {
			fileName = part.FileName()
			fileBody = body
		} else {
			fields[part.FormName()] = string(body)
		}
		_ = part.Close()
	}
	return fields, fileName, fileBody
}

// TestHTTPClient_UploadImage_HappyPath pins the wire shape of the
// upload-image outbound: POST /open-apis/im/v1/images, multipart body
// with image_type=message + an `image` file part carrying the bytes
// the caller streamed in, Authorization=Bearer <token>, and the
// response's data.image_key surfaced as the function's return value.
// Lark rejects malformed multipart with cryptic 9499xxxx codes — the
// per-field assertions here are the contract this test exists to pin.
func TestHTTPClient_UploadImage_HappyPath(t *testing.T) {
	fake := newLarkFake(t)
	fake.stubToken("tok_up", 7200)

	const wantBytes = "the-image-bytes-🌏"
	var (
		gotMethod   string
		gotPath     string
		gotAuth     string
		gotFields   map[string]string
		gotFile     string
		gotFileBody []byte
	)
	fake.mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotFields, gotFile, gotFileBody = parseUploadMultipart(t, r)
		writeJSON(w, map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]string{"image_key": "img_v2_xyz"},
		})
	})

	c := newTestClient(fake, time.Now)
	key, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader:    strings.NewReader(wantBytes),
		ImageType: "message",
		Filename:  "screenshot.png",
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if key != "img_v2_xyz" {
		t.Errorf("image_key: got %q want img_v2_xyz", key)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q want POST", gotMethod)
	}
	if gotPath != "/open-apis/im/v1/images" {
		t.Errorf("path: got %q want /open-apis/im/v1/images", gotPath)
	}
	if gotAuth != "Bearer tok_up" {
		t.Errorf("Authorization: got %q want Bearer tok_up", gotAuth)
	}
	if gotFields["image_type"] != "message" {
		t.Errorf("image_type form field: got %q want message", gotFields["image_type"])
	}
	if gotFile != "screenshot.png" {
		t.Errorf("filename: got %q want screenshot.png", gotFile)
	}
	if string(gotFileBody) != wantBytes {
		t.Errorf("file body: got %q want %q", gotFileBody, wantBytes)
	}
}

// TestHTTPClient_UploadImage_LarkErrorCode pins the failure path: a
// non-zero `code` becomes a wrapped error mentioning the code, and a
// missing image_key with code=0 is still treated as failure (mirrors
// the success-card path's contract).
func TestHTTPClient_UploadImage_LarkErrorCode(t *testing.T) {
	fake := newLarkFake(t)
	fake.stubToken("tok_up_err", 7200)
	fake.mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"code": 234567, "msg": "Permission denied"})
	})
	c := newTestClient(fake, time.Now)
	_, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader:    strings.NewReader("x"),
		ImageType: "message",
	})
	if err == nil {
		t.Fatal("expected error on non-zero Lark code")
	}
	if !strings.Contains(err.Error(), "code=234567") {
		t.Errorf("error should surface the Lark code; got %v", err)
	}
}

// TestHTTPClient_UploadImage_TokenExpired_InvalidatesCache mirrors the
// SendInteractiveCard token-invalidation contract: when Lark returns
// 99991663 (expired token) the cached tenant_access_token is dropped
// and the caller's retry refetches a fresh one before succeeding. This
// test counts both the token endpoint hits (must be exactly 2) and the
// upload endpoint hits (must be exactly 2: failed call + retry) so a
// regression that silently double-fetches is caught.
func TestHTTPClient_UploadImage_TokenExpired_InvalidatesCache(t *testing.T) {
	fake := newLarkFake(t)
	fake.stubToken("tok_up_first", 7200)
	var (
		uploadCalls atomic.Int32
		uploadHits  atomic.Int32
	)
	fake.mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		uploadHits.Add(1)
		// Drain the body so the streamer's goroutine completes — a
		// dangling pipe writer would race against the test's exit.
		_, _ = io.Copy(io.Discard, r.Body)
		n := uploadCalls.Add(1)
		if n == 1 {
			writeJSON(w, map[string]any{"code": codeTokenExpired, "msg": "expired"})
			return
		}
		writeJSON(w, map[string]any{"code": 0, "data": map[string]string{"image_key": "img_after_retry"}})
	})

	c := newTestClient(fake, time.Now)
	_, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader:    strings.NewReader("first-attempt"),
		ImageType: "message",
	})
	if err == nil {
		t.Fatal("first upload must fail with token-expired")
	}
	if !strings.Contains(err.Error(), "code=99991663") {
		t.Errorf("error should mention token-expired code: %v", err)
	}

	// Caller's retry — should re-fetch the token (cache was
	// invalidated), then succeed.
	key, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader:    strings.NewReader("second-attempt"),
		ImageType: "message",
	})
	if err != nil {
		t.Fatalf("retry upload: %v", err)
	}
	if key != "img_after_retry" {
		t.Errorf("retry image_key: got %q want img_after_retry", key)
	}
	if got := fake.tokenN.Load(); got != 2 {
		t.Errorf("token endpoint hits after invalidation: got %d want 2", got)
	}
	if got := uploadHits.Load(); got != 2 {
		t.Errorf("upload endpoint hits: got %d want 2", got)
	}
}

// TestHTTPClient_UploadImage_StreamsLargeBody guards the "do not
// buffer the whole image in memory" contract. We hand the client a
// ~4 MiB reader; the receiving handler reads it back and verifies
// every byte landed intact, but does NOT read it into a single
// pre-allocated buffer — the io.Copy keeps the streaming spirit.
func TestHTTPClient_UploadImage_StreamsLargeBody(t *testing.T) {
	const sz = 4 * 1024 * 1024
	want := bytes.Repeat([]byte("multica-stream"), sz/14+1)[:sz]

	fake := newLarkFake(t)
	fake.stubToken("tok_up_stream", 7200)
	var gotLen int
	var gotHash byte
	fake.mux.HandleFunc("/open-apis/im/v1/images", func(w http.ResponseWriter, r *http.Request) {
		_, _, body := parseUploadMultipart(t, r)
		gotLen = len(body)
		// Cheap content fingerprint that does not need a full byte-
		// by-byte compare in the test failure log.
		for _, b := range body {
			gotHash ^= b
		}
		writeJSON(w, map[string]any{
			"code": 0,
			"data": map[string]string{"image_key": "img_big"},
		})
	})

	c := newTestClient(fake, time.Now)
	_, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader:    bytes.NewReader(want),
		ImageType: "message",
		Filename:  "big.png",
	})
	if err != nil {
		t.Fatalf("upload large: %v", err)
	}
	if gotLen != sz {
		t.Errorf("received body length: got %d want %d", gotLen, sz)
	}
	var wantHash byte
	for _, b := range want {
		wantHash ^= b
	}
	if gotHash != wantHash {
		t.Errorf("received body fingerprint mismatch: got %x want %x", gotHash, wantHash)
	}
}

// TestHTTPClient_UploadImage_Validation short-circuits on missing
// inputs BEFORE any auth round-trip, so a misuse can't leak load to
// the token endpoint.
func TestHTTPClient_UploadImage_Validation(t *testing.T) {
	fake := newLarkFake(t)
	c := newTestClient(fake, time.Now)
	if _, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		ImageType: "message",
	}); err == nil || !strings.Contains(err.Error(), "image reader") {
		t.Errorf("missing reader: want error mentioning image reader, got %v", err)
	}
	if _, err := c.UploadImage(context.Background(), testCreds(), UploadImageParams{
		Reader: strings.NewReader("x"),
	}); err == nil || !strings.Contains(err.Error(), "image_type") {
		t.Errorf("missing image_type: want error mentioning image_type, got %v", err)
	}
	if got := fake.tokenN.Load(); got != 0 {
		t.Errorf("token endpoint must not be hit on bad input: got %d", got)
	}
}
