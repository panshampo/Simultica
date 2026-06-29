package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// maxTaskAttachmentSize is the per-file ceiling for the daemon → server
// attachment upload path. The parent issue calls out >10MB as the rejection
// threshold; mirrored here so a renderer that emits a giant PNG can't be
// silently uploaded and then fail somewhere downstream.
const maxTaskAttachmentSize = 10 << 20 // 10 MiB

// taskAttachmentExtContentTypes overrides http.DetectContentType for
// extensions where the sniffer is unreliable for our supported image set.
var taskAttachmentExtContentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
}

type taskAttachmentResponse struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	MessageID  string `json:"message_id,omitempty"`
	Kind       string `json:"kind"`
	MimeType   string `json:"mime_type"`
	ByteSize   int64  `json:"byte_size"`
	StorageKey string `json:"storage_key"`
	URL        string `json:"url,omitempty"`
}

// UploadTaskAttachment receives a daemon-uploaded file (typically a chart PNG
// referenced from the task's final markdown), persists it through the
// configured storage backend, and records a task_message_attachment row that
// later stages (Lark card patcher, frontend timeline) can look up by task_id.
//
// The handler is intentionally narrow: only image/png and image/jpeg are
// accepted, anything bigger than maxTaskAttachmentSize is rejected up-front
// (10 MiB), and the storage key is derived from a server-generated UUID so the
// daemon-supplied filename can never traverse out of the storage namespace.
func (h *Handler) UploadTaskAttachment(w http.ResponseWriter, r *http.Request) {
	if h.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "file upload not configured")
		return
	}

	taskID := chi.URLParam(r, "taskId")
	task, ok := h.requireDaemonTaskAccess(w, r, taskID)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTaskAttachmentSize+1<<10)
	if err := r.ParseMultipartForm(maxTaskAttachmentSize + 1<<10); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("missing file field: %v", err))
		return
	}
	defer file.Close()

	if header.Size > maxTaskAttachmentSize {
		writeError(w, http.StatusRequestEntityTooLarge, "attachment exceeds 10MB limit")
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, maxTaskAttachmentSize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	if int64(len(data)) > maxTaskAttachmentSize {
		writeError(w, http.StatusRequestEntityTooLarge, "attachment exceeds 10MB limit")
		return
	}

	mimeType := http.DetectContentType(data)
	ext := strings.ToLower(path.Ext(header.Filename))
	if override, found := taskAttachmentExtContentTypes[ext]; found {
		mimeType = override
	}
	if mimeType != "image/png" && mimeType != "image/jpeg" {
		writeError(w, http.StatusUnsupportedMediaType, "only PNG and JPEG are accepted")
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("failed to generate task attachment id", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	storageKey := fmt.Sprintf("tasks/%s/%s%s", taskID, id.String(), ext)

	if _, err := h.Storage.Upload(r.Context(), storageKey, data, mimeType, header.Filename); err != nil {
		slog.Error("task attachment upload failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store attachment")
		return
	}

	var messageID pgtype.UUID
	if raw := strings.TrimSpace(r.FormValue("message_id")); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid message_id")
			return
		}
		messageID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	row, err := h.Queries.CreateTaskMessageAttachment(r.Context(), db.CreateTaskMessageAttachmentParams{
		TaskID:     task.ID,
		MessageID:  messageID,
		Kind:       "image",
		MimeType:   mimeType,
		ByteSize:   int64(len(data)),
		StorageKey: storageKey,
	})
	if err != nil {
		slog.Error("create task_message_attachment failed", "task_id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to record attachment")
		return
	}

	resp := taskAttachmentResponse{
		ID:         uuidToString(row.ID),
		TaskID:     uuidToString(row.TaskID),
		MessageID:  uuidToString(row.MessageID),
		Kind:       row.Kind,
		MimeType:   row.MimeType,
		ByteSize:   row.ByteSize,
		StorageKey: row.StorageKey,
	}
	writeJSON(w, http.StatusCreated, resp)
}
