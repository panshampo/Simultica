package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUploadTaskAttachment_StorageDisabled exercises the storage-nil
// short-circuit, which runs before any DB access. Avoids the
// DB-fixture dependency of the rest of this package's tests.
func TestUploadTaskAttachment_StorageDisabled(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/daemon/tasks/00000000-0000-0000-0000-000000000000/attachments", nil)
	rr := httptest.NewRecorder()

	h.UploadTaskAttachment(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when Storage is nil, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}
