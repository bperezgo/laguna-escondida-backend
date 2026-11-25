package httpclient

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testReleaseRepo = "owner/repo"

const testReleaseList = `[
  {"tag_name": "edge/v1.5.0", "draft": false, "prerelease": true},
  {"tag_name": "landing/v2.0.0", "draft": false, "prerelease": false},
  {"tag_name": "cloud/v1.9.0", "draft": false, "prerelease": false},
  {"tag_name": "edge/v1.4.0", "draft": false, "prerelease": false},
  {"tag_name": "edge/v1.3.0-rc1", "draft": false, "prerelease": false},
  {"tag_name": "edge/v1.2.0", "draft": true, "prerelease": false}
]`

// releaseServer stands in for both api.github.com and github.com: paths maps a path to its body.
func releaseServer(t *testing.T, paths map[string]string) (*GitHubReleaseCatalog, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := paths[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newGitHubReleaseCatalog(&Client{Client: server.Client()}, server.URL, server.URL, testReleaseRepo, logger), server.URL
}

func manifestPath(version string) string {
	return "/" + testReleaseRepo + "/releases/download/edge/" + version + "/manifest.json"
}

func TestGitHubReleaseCatalog_ListEdgeReleases_KeepsOnlyEdgeVersions(t *testing.T) {
	catalog, _ := releaseServer(t, map[string]string{"/repos/" + testReleaseRepo + "/releases": testReleaseList})

	releases, err := catalog.ListEdgeReleases(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []dto.EdgeRelease{
		{Version: "v1.5.0", Prerelease: true},
		{Version: "v1.4.0"},
		{Version: "v1.2.0", Draft: true},
	}, releases)
}

func TestGitHubReleaseCatalog_ListEdgeReleases_UpstreamErrorIsReturned(t *testing.T) {
	catalog, _ := releaseServer(t, map[string]string{})

	_, err := catalog.ListEdgeReleases(context.Background())

	assert.Error(t, err)
}

func TestGitHubReleaseCatalog_GetEdgeManifest(t *testing.T) {
	// The server's own URL is the download base, so a well-formed installer address lives under it.
	paths := map[string]string{}
	catalog, base := releaseServer(t, paths)
	installer := base + "/" + testReleaseRepo + "/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe"
	paths[manifestPath("v1.4.0")] = `{"min_cloud": "v1.3.0", "components": {"installer": {"version": "v1.4.0", "url": "` + installer + `", "sha256": "ab"}}}`
	paths[manifestPath("v1.3.0")] = `{"min_cloud": "v1.2.0", "components": {"edge-node": {"url": "x"}}}`
	paths[manifestPath("v1.2.0")] = `{"min_cloud": "v1.2.0", "components": {"installer": {"url": "https://example.com/KuentaSetup.exe"}}}`
	paths[manifestPath("v1.1.0")] = `{"min_cloud": "v1.2.0", "components": {"installer": {"url": "` + base + `/other/repo/releases/download/edge/v1.1.0/KuentaSetup.exe"}}}`
	paths[manifestPath("v1.0.0")] = `not json`

	t.Run("normal manifest", func(t *testing.T) {
		got, err := catalog.GetEdgeManifest(context.Background(), "v1.4.0")
		require.NoError(t, err)
		assert.Equal(t, &dto.EdgeManifest{MinCloud: "v1.3.0", InstallerURL: installer}, got)
	})
	t.Run("manifest without an installer", func(t *testing.T) {
		got, err := catalog.GetEdgeManifest(context.Background(), "v1.3.0")
		require.NoError(t, err)
		assert.Empty(t, got.InstallerURL)
	})
	t.Run("foreign installer address is dropped", func(t *testing.T) {
		got, err := catalog.GetEdgeManifest(context.Background(), "v1.2.0")
		require.NoError(t, err)
		assert.Empty(t, got.InstallerURL)
	})
	t.Run("another repository's download is dropped", func(t *testing.T) {
		got, err := catalog.GetEdgeManifest(context.Background(), "v1.1.0")
		require.NoError(t, err)
		assert.Empty(t, got.InstallerURL)
	})
	t.Run("manifest that cannot be parsed", func(t *testing.T) {
		_, err := catalog.GetEdgeManifest(context.Background(), "v1.0.0")
		assert.Error(t, err)
	})
	t.Run("manifest that fails to load", func(t *testing.T) {
		_, err := catalog.GetEdgeManifest(context.Background(), "v0.9.0")
		assert.Error(t, err)
	})
}

func TestGitHubReleaseCatalog_IsReleaseDownload(t *testing.T) {
	catalog := newGitHubReleaseCatalog(nil, githubAPIBaseURL, githubDownloadBaseURL, testReleaseRepo, nil)
	cases := map[string]bool{
		"https://github.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe":     true,
		"http://github.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup-v1.4.0.exe":      false,
		"https://github.com/owner/repo-evil/releases/download/edge/v1.4.0/KuentaSetup.exe":       false,
		"https://github.com.evil.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup.exe":   false,
		"https://github.com/owner/repo/releases/download/../../../other/repo/KuentaSetup.exe":    false,
		"https://user@github.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup.exe":       false,
		"https://github.com/owner/repo/releases/download/edge/v1.4.0/KuentaSetup.exe?redirect=x": false,
		"https://example.com/KuentaSetup.exe":                                                    false,
	}
	for raw, want := range cases {
		assert.Equal(t, want, catalog.isReleaseDownload(raw), raw)
	}
}
