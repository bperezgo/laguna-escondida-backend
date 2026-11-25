package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_EdgeReleasesRepoDefaultsWhenUnset(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("EDGE_RELEASES_REPO", "")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, DefaultEdgeReleasesRepo, cfg.EdgeReleasesRepo)
}

func TestLoadCloud_EdgeReleasesRepoValidValue(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("EDGE_RELEASES_REPO", " some-org/kuenta.releases ")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.Equal(t, "some-org/kuenta.releases", cfg.EdgeReleasesRepo)
}

func TestLoadCloud_EdgeReleasesRepoMalformedStopsStartup(t *testing.T) {
	for _, value := range []string{"just-a-name", "owner/name/extra", "owner/..", "https://github.com/owner/name", "owner/na me", "/name"} {
		t.Run(value, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv("EDGE_RELEASES_REPO", value)

			_, err := LoadCloud()

			require.Error(t, err)
			assert.Contains(t, err.Error(), "EDGE_RELEASES_REPO")
		})
	}
}
