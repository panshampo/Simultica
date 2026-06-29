package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// markdownImageRef matches Markdown image syntax: ![alt](path "title").
// We only care about the URL/path portion; alt and title are ignored.
// The pattern is intentionally conservative: it stops at the first
// whitespace or closing paren so a malformed link can't gobble the rest
// of the document.
var markdownImageRef = regexp.MustCompile(`!\[[^\]]*\]\(([^)\s]+)`)

// imageExtMime maps the supported image extensions to their MIME type.
// Matches the server-side allow-list (PNG and JPEG).
var imageExtMime = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
}

// uploadFinalImageAttachments scans the agent's final markdown comment for
// local image references and uploads each one through the daemon's
// attachment endpoint. Behavior is best-effort by design:
//
//   - errors per file are logged at WARN level and DO NOT block the task
//     final (per the parent issue's "warn log, do not block task final").
//   - oversized files (>10 MiB) are skipped client-side without ever
//     hitting the server, matching the server's hard cap.
//   - non-image references, absolute URLs (http(s)://), and absolute
//     filesystem paths outside the agent workdir are skipped.
//
// Returns the list of successfully uploaded attachment responses so a
// future caller (the Lark patcher in P2) can correlate them back to the
// markdown without re-querying the DB.
func (d *Daemon) uploadFinalImageAttachments(ctx context.Context, taskID, workDir, comment string, taskLog *slog.Logger) []TaskAttachmentResponse {
	if comment == "" || workDir == "" {
		return nil
	}
	matches := markdownImageRef.FindAllStringSubmatch(comment, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(matches))
	var uploaded []TaskAttachmentResponse
	for _, m := range matches {
		ref := strings.TrimSpace(m[1])
		if ref == "" {
			continue
		}
		// Skip remote references — these are already URLs the renderer can
		// hot-link directly. The upload channel is for local files only.
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			continue
		}
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}

		ext := strings.ToLower(filepath.Ext(ref))
		mime, ok := imageExtMime[ext]
		if !ok {
			continue
		}

		abs := ref
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workDir, ref)
		}
		// Anti-traversal: refuse paths that escape the agent's workdir.
		// Without this, a `![](../../etc/passwd)` reference (or just a
		// careless `../shared.png`) would let the daemon read arbitrary
		// files off the user's machine and post them to the server.
		cleanedAbs, err := filepath.Abs(abs)
		if err != nil {
			taskLog.Warn("attachment upload: failed to resolve path", "ref", ref, "error", err)
			continue
		}
		cleanedRoot, err := filepath.Abs(workDir)
		if err != nil {
			taskLog.Warn("attachment upload: failed to resolve workdir", "work_dir", workDir, "error", err)
			continue
		}
		rel, err := filepath.Rel(cleanedRoot, cleanedAbs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			taskLog.Warn("attachment upload: refusing to read outside workdir", "ref", ref)
			continue
		}

		data, err := os.ReadFile(cleanedAbs)
		if err != nil {
			taskLog.Warn("attachment upload: read failed", "ref", ref, "error", err)
			continue
		}
		if int64(len(data)) > MaxTaskAttachmentSize {
			taskLog.Warn("attachment upload: file exceeds 10MB cap, skipping", "ref", ref, "bytes", len(data))
			continue
		}

		filename := filepath.Base(cleanedAbs)
		resp, err := d.client.UploadTaskAttachment(ctx, taskID, filename, mime, data, "")
		if err != nil {
			if errors.Is(err, ErrTaskAttachmentTooLarge) {
				taskLog.Warn("attachment upload: rejected by client cap", "ref", ref, "bytes", len(data))
				continue
			}
			taskLog.Warn("attachment upload: server rejected", "ref", ref, "error", err)
			continue
		}
		taskLog.Info("attachment uploaded", "ref", ref, "attachment_id", resp.ID, "bytes", resp.ByteSize)
		uploaded = append(uploaded, *resp)
	}

	if len(uploaded) > 0 {
		taskLog.Info(fmt.Sprintf("uploaded %d task attachment(s)", len(uploaded)), "task_id", taskID)
	}
	return uploaded
}
