package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

var releaseVersionPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// InstallerDownloadService picks the installer with the rule the boxes' updater applies
// (edge/scripts/releases.ps1), so a new box and an updated box land on the same release.
type InstallerDownloadService struct {
	catalog      ports.EdgeReleaseCatalog
	cloudVersion string
}

func NewInstallerDownloadService(catalog ports.EdgeReleaseCatalog, cloudVersion string) *InstallerDownloadService {
	return &InstallerDownloadService{catalog: catalog, cloudVersion: cloudVersion}
}

// Current returns the newest installer the cloud can sync with, or ErrInstallerUnavailable.
func (s *InstallerDownloadService) Current(ctx context.Context) (*dto.EdgeInstaller, error) {
	releases, err := s.catalog.ListEdgeReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: listing edge releases: %w", domainError.ErrInstallerUnavailable, err)
	}
	cloud, cloudIsRelease := parseReleaseVersion(s.cloudVersion)

	type candidate struct {
		version string
		parsed  [3]int
	}
	var candidates []candidate
	for _, r := range releases {
		parsed, ok := parseReleaseVersion(r.Version)
		if !ok || r.Draft || r.Prerelease {
			continue
		}
		candidates = append(candidates, candidate{version: r.Version, parsed: parsed})
	}
	sort.Slice(candidates, func(i, j int) bool { return releaseVersionLess(candidates[j].parsed, candidates[i].parsed) })

	for _, c := range candidates {
		manifest, err := s.catalog.GetEdgeManifest(ctx, c.version)
		if err != nil || manifest == nil || manifest.InstallerURL == "" {
			continue
		}
		// A development cloud has no version to compare with, as on a first install.
		if cloudIsRelease {
			minCloud, ok := parseReleaseVersion(manifest.MinCloud)
			if !ok || releaseVersionLess(cloud, minCloud) {
				continue
			}
		}
		return &dto.EdgeInstaller{Version: c.version, URL: manifest.InstallerURL}, nil
	}
	return nil, domainError.ErrInstallerUnavailable
}

func parseReleaseVersion(s string) ([3]int, bool) {
	m := releaseVersionPattern.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := range v {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

func releaseVersionLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
