package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	githubAPIBaseURL      = "https://api.github.com"
	githubDownloadBaseURL = "https://github.com"
	releaseCatalogTimeout = 10 * time.Second
	maxReleaseListBytes   = 8 << 20
	maxManifestBytes      = 256 << 10
)

var edgeTagPattern = regexp.MustCompile(`^edge/(v\d+\.\d+\.\d+)$`)

// GitHubReleaseCatalog reads edge releases from a public GitHub repository, without a token.
type GitHubReleaseCatalog struct {
	client       *Client
	apiBase      string
	downloadBase string
	repo         string
	logger       *slog.Logger
}

func NewGitHubReleaseCatalog(client *Client, repo string, logger *slog.Logger) ports.EdgeReleaseCatalog {
	return newGitHubReleaseCatalog(client, githubAPIBaseURL, githubDownloadBaseURL, repo, logger)
}

func newGitHubReleaseCatalog(client *Client, apiBase, downloadBase, repo string, logger *slog.Logger) *GitHubReleaseCatalog {
	return &GitHubReleaseCatalog{
		client: client, apiBase: strings.TrimRight(apiBase, "/"), downloadBase: strings.TrimRight(downloadBase, "/"),
		repo: repo, logger: logger,
	}
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type edgeManifestFile struct {
	MinCloud   string `json:"min_cloud"`
	Components struct {
		Installer *struct {
			URL string `json:"url"`
		} `json:"installer"`
	} `json:"components"`
}

func (c *GitHubReleaseCatalog) ListEdgeReleases(ctx context.Context) ([]dto.EdgeRelease, error) {
	body, err := c.fetch(ctx, fmt.Sprintf("%s/repos/%s/releases?per_page=50", c.apiBase, c.repo), maxReleaseListBytes)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("parse release list: %w", err)
	}
	var edge []dto.EdgeRelease
	for _, r := range releases {
		m := edgeTagPattern.FindStringSubmatch(r.TagName)
		if m == nil {
			continue
		}
		edge = append(edge, dto.EdgeRelease{Version: m[1], Draft: r.Draft, Prerelease: r.Prerelease})
	}
	return edge, nil
}

func (c *GitHubReleaseCatalog) GetEdgeManifest(ctx context.Context, version string) (*dto.EdgeManifest, error) {
	body, err := c.fetch(ctx, c.releaseDownloadPrefix()+"edge/"+version+"/manifest.json", maxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("read manifest of edge/%s: %w", version, err)
	}
	var file edgeManifestFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("parse manifest of edge/%s: %w", version, err)
	}
	manifest := &dto.EdgeManifest{MinCloud: file.MinCloud}
	if file.Components.Installer == nil || file.Components.Installer.URL == "" {
		return manifest, nil
	}
	if !c.isReleaseDownload(file.Components.Installer.URL) {
		// Never an open redirect, even if a release asset were replaced.
		c.logger.Warn("Edge manifest names an installer outside the release downloads; release skipped",
			"version", version, "installer_url", file.Components.Installer.URL)
		return manifest, nil
	}
	manifest.InstallerURL = file.Components.Installer.URL
	return manifest, nil
}

func (c *GitHubReleaseCatalog) releaseDownloadPrefix() string {
	return c.downloadBase + "/" + c.repo + "/releases/download/"
}

func (c *GitHubReleaseCatalog) isReleaseDownload(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		strings.Contains(parsed.Path, "..") {
		return false
	}
	return strings.HasPrefix(parsed.String(), c.releaseDownloadPrefix())
}

// fetch reads a body of at most limit bytes; a larger one is an error rather than a truncated document.
func (c *GitHubReleaseCatalog) fetch(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, releaseCatalogTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("body larger than %d bytes", limit)
	}
	return body, nil
}
