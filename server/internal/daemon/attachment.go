package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// MaxTaskAttachmentSize matches the server-side cap (10 MiB). The daemon
// pre-checks this before constructing the multipart body so a renderer
// emitting a giant PNG is rejected without spending bytes on the upload.
const MaxTaskAttachmentSize = 10 << 20

// ErrTaskAttachmentTooLarge is returned by UploadTaskAttachment when the
// caller passes data exceeding MaxTaskAttachmentSize. Callers downgrade the
// upload to a warn log; the parent issue spec is "warn log, do not block
// task final".
var ErrTaskAttachmentTooLarge = errors.New("task attachment exceeds 10MB limit")

// TaskAttachmentResponse mirrors the JSON shape returned by the server's
// POST /api/daemon/tasks/{task_id}/attachments handler.
type TaskAttachmentResponse struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	MessageID  string `json:"message_id,omitempty"`
	Kind       string `json:"kind"`
	MimeType   string `json:"mime_type"`
	ByteSize   int64  `json:"byte_size"`
	StorageKey string `json:"storage_key"`
}

// UploadTaskAttachment streams a single image attachment for the given task
// to the server using a multipart/form-data POST. The server enforces its
// own 10MB cap and PNG/JPEG-only contract; this client mirrors the size
// check so clearly-oversized files never hit the wire.
//
// filename is forwarded verbatim as the form file name (best-effort hint
// for storage backends with sidecar metadata); messageID is optional and
// pins the row to a specific task_message when known.
func (c *Client) UploadTaskAttachment(ctx context.Context, taskID, filename, mimeType string, data []byte, messageID string) (*TaskAttachmentResponse, error) {
	if int64(len(data)) > MaxTaskAttachmentSize {
		return nil, ErrTaskAttachmentTooLarge
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if messageID != "" {
		if err := mw.WriteField("message_id", messageID); err != nil {
			return nil, fmt.Errorf("write message_id field: %w", err)
		}
	}
	header := make(map[string][]string, 2)
	header["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="file"; filename=%q`, filename),
	}
	if mimeType != "" {
		header["Content-Type"] = []string{mimeType}
	}
	part, err := mw.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write multipart part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	path := fmt.Sprintf("/api/daemon/tasks/%s/attachments", taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	c.setIdentityHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &requestError{Method: http.MethodPost, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(buf))}
	}

	var out TaskAttachmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode upload response: %w", err)
	}
	return &out, nil
}
