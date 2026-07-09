package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	skillpkg "github.com/multica-ai/multica/server/internal/skill"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"gopkg.in/yaml.v3"
)

const originTypeGlobalLink = "global_link"

type CreateGlobalSkillLinkRequest struct {
	TargetPath  string  `json:"target_path"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type SkillHealthResponse struct {
	Status       string   `json:"status"`
	Reasons      []string `json:"reasons"`
	ResolvedPath *string  `json:"resolved_path,omitempty"`
	ContentHash  *string  `json:"content_hash,omitempty"`
	UpdatedAt    *string  `json:"updated_at,omitempty"`
	HasWorkflow  bool     `json:"has_workflow"`
}

type ResolvedSkillBundleResponse struct {
	SkillID     string              `json:"skill_id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	SourceType  string              `json:"source_type"`
	Content     string              `json:"content"`
	Files       []SkillFileResponse `json:"files"`
	Workflow    *SkillFileResponse  `json:"workflow,omitempty"`
	Health      SkillHealthResponse `json:"health"`
}

type SkillWorkflowValidationResponse struct {
	Health     SkillHealthResponse `json:"health"`
	Validation any                 `json:"validation,omitempty"`
	Error      string              `json:"error,omitempty"`
}

func enrichSkillConfig(raw []byte) any {
	configMap := decodeSkillConfigMap(raw)
	health := skillHealthFromConfig(configMap)
	if health != nil {
		configMap["health"] = health
		configMap["source_type"] = skillOriginType(configMap)
	}
	return configMap
}

func (h *Handler) skillConfigForResponse(skill db.Skill) map[string]any {
	configMap := decodeSkillConfigMap(skill.Config)
	if origin, health, ok := h.workspaceGlobalLinkOverlay(skill.Name); ok {
		configMap["origin"] = origin
		configMap["has_workflow"] = health.HasWorkflow
		configMap["health"] = skillHealthToConfig(health)
		configMap["source_type"] = originTypeGlobalLink
		return configMap
	}
	health := skillHealthFromConfig(configMap)
	if health != nil {
		configMap["health"] = health
		configMap["source_type"] = skillOriginType(configMap)
	}
	return configMap
}

func (h *Handler) workspaceGlobalLinkOverlay(name string) (map[string]any, SkillHealthResponse, bool) {
	linkPath, err := workspaceSkillLinkPath(name)
	if err != nil {
		return nil, SkillHealthResponse{}, false
	}
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil, SkillHealthResponse{}, false
	}
	bundle, err := resolveGlobalSkillLinkBundle(linkPath)
	if err != nil {
		health := SkillHealthResponse{Status: "broken", Reasons: []string{err.Error()}}
		return map[string]any{
			"type":        originTypeGlobalLink,
			"target_kind": "global_skill_library",
			"target_path": linkPath,
			"health": map[string]any{
				"status":  health.Status,
				"reasons": health.Reasons,
			},
		}, health, true
	}
	resolvedPath := bundle.resolvedPath
	contentHash := bundle.contentHash
	updatedAt := bundle.updatedAt
	health := SkillHealthResponse{
		Status:       "ok",
		Reasons:      []string{},
		ResolvedPath: &resolvedPath,
		ContentHash:  &contentHash,
		UpdatedAt:    &updatedAt,
		HasWorkflow:  bundle.hasWorkflow,
	}
	return map[string]any{
		"type":          originTypeGlobalLink,
		"target_kind":   "global_skill_library",
		"target_path":   linkPath,
		"resolved_path": bundle.resolvedPath,
		"content_hash":  bundle.contentHash,
		"updated_at":    bundle.updatedAt,
		"health": map[string]any{
			"status":  "ok",
			"reasons": []string{},
		},
	}, health, true
}

func decodeSkillConfigMap(raw []byte) map[string]any {
	config := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &config)
	}
	return config
}

func skillOriginType(config map[string]any) string {
	if origin, ok := config["origin"].(map[string]any); ok {
		if typ, ok := origin["type"].(string); ok && typ != "" {
			return typ
		}
	}
	return "manual"
}

func skillHealthFromConfig(config map[string]any) map[string]any {
	origin, ok := config["origin"].(map[string]any)
	if !ok || origin["type"] != originTypeGlobalLink {
		return nil
	}
	status := "ok"
	reasons := []string{}
	if h, ok := origin["health"].(map[string]any); ok {
		if got, ok := h["status"].(string); ok && got != "" {
			status = got
		}
		if rawReasons, ok := h["reasons"].([]any); ok {
			for _, r := range rawReasons {
				if s, ok := r.(string); ok && s != "" {
					reasons = append(reasons, s)
				}
			}
		}
	}
	out := map[string]any{
		"status":       status,
		"reasons":      reasons,
		"has_workflow": config["has_workflow"] == true,
	}
	for _, key := range []string{"resolved_path", "content_hash", "updated_at"} {
		if v, ok := origin[key].(string); ok && v != "" {
			out[key] = v
		}
	}
	return out
}

func skillHealthToConfig(health SkillHealthResponse) map[string]any {
	out := map[string]any{
		"status":       health.Status,
		"reasons":      health.Reasons,
		"has_workflow": health.HasWorkflow,
	}
	if health.ResolvedPath != nil {
		out["resolved_path"] = *health.ResolvedPath
	}
	if health.ContentHash != nil {
		out["content_hash"] = *health.ContentHash
	}
	if health.UpdatedAt != nil {
		out["updated_at"] = *health.UpdatedAt
	}
	return out
}

func (h *Handler) createGlobalSkillLink(ctx context.Context, workspaceID, creatorID pgtype.UUID, req CreateGlobalSkillLinkRequest) (SkillWithFilesResponse, error) {
	bundle, err := resolveGlobalSkillLinkBundle(req.TargetPath)
	if err != nil {
		return SkillWithFilesResponse{}, err
	}
	name := bundle.name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = strings.TrimSpace(*req.Name)
	}
	description := bundle.description
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	config := map[string]any{
		"has_workflow":        bundle.hasWorkflow,
		"workflow_validation": workflowValidationFromFiles(bundle.files),
		"origin": map[string]any{
			"type":          originTypeGlobalLink,
			"target_kind":   "global_skill_library",
			"target_path":   bundle.targetPath,
			"resolved_path": bundle.resolvedPath,
			"content_hash":  bundle.contentHash,
			"updated_at":    bundle.updatedAt,
			"health": map[string]any{
				"status":  "ok",
				"reasons": []string{},
			},
		},
	}

	return h.createSkillWithFiles(ctx, skillCreateInput{
		WorkspaceID: workspaceID,
		CreatorID:   creatorID,
		Name:        name,
		Description: description,
		Content:     bundle.content,
		Config:      config,
		Files:       bundle.files,
	})
}

type globalSkillLinkBundle struct {
	name         string
	description  string
	content      string
	files        []CreateSkillFileRequest
	targetPath   string
	resolvedPath string
	contentHash  string
	updatedAt    string
	hasWorkflow  bool
}

func resolveGlobalSkillLinkBundle(targetPath string) (globalSkillLinkBundle, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return globalSkillLinkBundle{}, errors.New("target_path is required")
	}
	resolved, err := resolveAllowedSkillLinkPath(targetPath)
	if err != nil {
		return globalSkillLinkBundle{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return globalSkillLinkBundle{}, fmt.Errorf("global link target not readable: %w", err)
	}
	if !info.IsDir() {
		return globalSkillLinkBundle{}, errors.New("global link target must be a directory")
	}
	contentBytes, err := os.ReadFile(filepath.Join(resolved, "SKILL.md"))
	if err != nil {
		return globalSkillLinkBundle{}, fmt.Errorf("global link target must contain readable SKILL.md: %w", err)
	}
	content := string(contentBytes)
	name, description := parseSkillFrontmatter(content)
	if name == "" {
		name = filepath.Base(resolved)
	}

	files, hash, latestMod, err := readGlobalSkillSupportingFiles(resolved, contentBytes)
	if err != nil {
		return globalSkillLinkBundle{}, err
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	if !latestMod.IsZero() {
		updatedAt = latestMod.UTC().Format(time.RFC3339)
	}
	return globalSkillLinkBundle{
		name:         name,
		description:  description,
		content:      content,
		files:        files,
		targetPath:   targetPath,
		resolvedPath: resolved,
		contentHash:  hash,
		updatedAt:    updatedAt,
		hasWorkflow:  fileListHasWorkflow(files),
	}, nil
}

func resolveAllowedSkillLinkPath(targetPath string) (string, error) {
	abs, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	allowedRoots := allowedGlobalSkillRoots()
	for _, root := range allowedRoots {
		if pathWithinRoot(resolved, root) {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("global link target %q is outside allowed skill roots", targetPath)
}

func allowedGlobalSkillRoots() []string {
	raw := strings.TrimSpace(os.Getenv("MULTICA_GLOBAL_SKILL_ROOTS"))
	roots := []string{}
	if raw != "" {
		for _, item := range strings.Split(raw, string(os.PathListSeparator)) {
			if item = strings.TrimSpace(item); item != "" {
				roots = append(roots, item)
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".agents", "skills"))
	}
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			root := filepath.Join(dir, ".agents", "skills")
			if _, err := os.Stat(root); err == nil {
				roots = append(roots, root)
			}
		}
	}
	resolved := []string{}
	seen := map[string]bool{}
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if r, err := filepath.EvalSymlinks(abs); err == nil {
			abs = r
		}
		if !seen[abs] {
			seen[abs] = true
			resolved = append(resolved, abs)
		}
	}
	return resolved
}

func workspaceSkillLinkPath(name string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		root := filepath.Join(dir, ".agents", "skills")
		if _, err := os.Stat(root); err == nil {
			return filepath.Join(root, name), nil
		}
	}
	return "", errors.New("workspace .agents/skills root not found")
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func readGlobalSkillSupportingFiles(root string, contentBytes []byte) ([]CreateSkillFileRequest, string, time.Time, error) {
	hasher := sha256.New()
	hasher.Write([]byte("SKILL.md\x00"))
	hasher.Write(contentBytes)
	files := []CreateSkillFileRequest{}
	var latestMod time.Time
	var totalSize int64 = int64(len(contentBytes))
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "SKILL.md" {
			if info, err := d.Info(); err == nil {
				latestMod = maxTime(latestMod, info.ModTime())
			}
			return nil
		}
		if !validateFilePath(rel) || skillpkg.IsReservedContentPath(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxImportFileSize {
			return fmt.Errorf("global link file %q exceeds %d bytes", rel, maxImportFileSize)
		}
		totalSize += info.Size()
		if totalSize > maxImportTotalSize {
			return fmt.Errorf("global link bundle exceeds %d bytes", maxImportTotalSize)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(files) >= maxImportFileCount {
			return fmt.Errorf("global link bundle exceeds %d files", maxImportFileCount)
		}
		files = append(files, CreateSkillFileRequest{Path: rel, Content: string(data)})
		hasher.Write([]byte(rel + "\x00"))
		hasher.Write(data)
		latestMod = maxTime(latestMod, info.ModTime())
		return nil
	})
	if err != nil {
		return nil, "", time.Time{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, hex.EncodeToString(hasher.Sum(nil)), latestMod, nil
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func parseSkillFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", ""
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", ""
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return "", ""
	}
	name, _ := fm["name"].(string)
	description, _ := fm["description"].(string)
	return strings.TrimSpace(name), strings.TrimSpace(description)
}

func (h *Handler) resolvedSkillBundle(ctx context.Context, skill db.Skill) ResolvedSkillBundleResponse {
	config := decodeSkillConfigMap(skill.Config)
	if origin, _, ok := h.workspaceGlobalLinkOverlay(skill.Name); ok {
		config["origin"] = origin
	}
	sourceType := skillOriginType(config)
	files := []SkillFileResponse{}
	health := SkillHealthResponse{Status: "ok", Reasons: []string{}}
	if sourceType == originTypeGlobalLink {
		origin, _ := config["origin"].(map[string]any)
		target, _ := origin["target_path"].(string)
		bundle, err := resolveGlobalSkillLinkBundle(target)
		if err != nil {
			health.Status = "broken"
			health.Reasons = []string{err.Error()}
			return ResolvedSkillBundleResponse{
				SkillID:     uuidToString(skill.ID),
				Name:        skill.Name,
				Description: skill.Description,
				SourceType:  sourceType,
				Content:     skill.Content,
				Files:       files,
				Health:      health,
			}
		}
		for _, f := range bundle.files {
			files = append(files, SkillFileResponse{SkillID: uuidToString(skill.ID), Path: f.Path, Content: f.Content})
		}
		resolvedPath := bundle.resolvedPath
		contentHash := bundle.contentHash
		updatedAt := bundle.updatedAt
		health.ResolvedPath = &resolvedPath
		health.ContentHash = &contentHash
		health.UpdatedAt = &updatedAt
		health.HasWorkflow = bundle.hasWorkflow
		return ResolvedSkillBundleResponse{
			SkillID:     uuidToString(skill.ID),
			Name:        skill.Name,
			Description: skill.Description,
			SourceType:  sourceType,
			Content:     bundle.content,
			Files:       files,
			Workflow:    workflowFileFromResponses(files),
			Health:      health,
		}
	}
	dbFiles, err := h.Queries.ListSkillFiles(ctx, skill.ID)
	if err == nil {
		for _, f := range dbFiles {
			files = append(files, skillFileToResponse(f))
		}
	}
	health.HasWorkflow = config["has_workflow"] == true
	return ResolvedSkillBundleResponse{
		SkillID:     uuidToString(skill.ID),
		Name:        skill.Name,
		Description: skill.Description,
		SourceType:  sourceType,
		Content:     skill.Content,
		Files:       files,
		Workflow:    workflowFileFromResponses(files),
		Health:      health,
	}
}

func workflowFileFromResponses(files []SkillFileResponse) *SkillFileResponse {
	for _, f := range files {
		if f.Path == workflowFilePath {
			copy := f
			return &copy
		}
	}
	return nil
}
