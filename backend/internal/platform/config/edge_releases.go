package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// DefaultEdgeReleasesRepo is the repository edge/scripts/releases.ps1 updates boxes from.
const DefaultEdgeReleasesRepo = "bperezgo/laguna-escondida-backend"

var repoSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// loadEdgeReleasesRepo reads EDGE_RELEASES_REPO ("owner/name"), the source of the installer download.
func loadEdgeReleasesRepo() (string, error) {
	raw := strings.TrimSpace(os.Getenv("EDGE_RELEASES_REPO"))
	if raw == "" {
		return DefaultEdgeReleasesRepo, nil
	}
	parts := strings.Split(raw, "/")
	valid := len(parts) == 2
	for _, p := range parts {
		valid = valid && repoSegment.MatchString(p) && p != "." && p != ".."
	}
	if !valid {
		return "", fmt.Errorf("invalid EDGE_RELEASES_REPO %q: want owner/name", raw)
	}
	return raw, nil
}
