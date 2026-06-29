package execenv

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

const externalDirectoryResourceType = "external_directory"

type externalDirectoryRef struct {
	LocalPath string `json:"local_path"`
	DaemonID  string `json:"daemon_id"`
}

func externalWritableRoots(resources []ProjectResourceForEnv, daemonID string) []string {
	daemonID = strings.TrimSpace(daemonID)
	if daemonID == "" {
		return nil
	}

	seen := map[string]struct{}{}
	for _, r := range resources {
		if r.ResourceType != externalDirectoryResourceType {
			continue
		}
		var ref externalDirectoryRef
		if err := json.Unmarshal(r.ResourceRef, &ref); err != nil {
			continue
		}
		if strings.TrimSpace(ref.DaemonID) != daemonID {
			continue
		}
		path := strings.TrimSpace(ref.LocalPath)
		if path == "" || !filepath.IsAbs(path) {
			continue
		}
		seen[filepath.Clean(path)] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}
