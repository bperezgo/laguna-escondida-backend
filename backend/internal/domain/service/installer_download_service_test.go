package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testReleaseDownloads = "https://github.com/owner/repo/releases/download/edge/"

func installerManifest(version, minCloud string) *dto.EdgeManifest {
	return &dto.EdgeManifest{MinCloud: minCloud, InstallerURL: testReleaseDownloads + version + "/KuentaSetup-" + version + ".exe"}
}

func newInstallerDownloadService(t *testing.T, cloudVersion string, releases []dto.EdgeRelease, manifests map[string]*dto.EdgeManifest) *InstallerDownloadService {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return(releases, nil)
	catalog.EXPECT().GetEdgeManifest(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, version string) (*dto.EdgeManifest, error) {
			if m, ok := manifests[version]; ok {
				return m, nil
			}
			return nil, errors.New("manifest not found")
		}).Maybe()
	return NewInstallerDownloadService(catalog, cloudVersion)
}

func TestInstallerDownloadService_Current_Selection(t *testing.T) {
	tests := []struct {
		name      string
		cloud     string
		releases  []dto.EdgeRelease
		manifests map[string]*dto.EdgeManifest
		want      string
	}{
		{
			name:     "newest release needs a newer cloud",
			cloud:    "v1.3.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": installerManifest("v1.5.0", "v1.4.0"),
				"v1.4.0": installerManifest("v1.4.0", "v1.3.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "prerelease is skipped",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0", Prerelease: true}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": installerManifest("v1.5.0", "v1.0.0"),
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "draft is skipped",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0", Draft: true}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": installerManifest("v1.5.0", "v1.0.0"),
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "release without an installer",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": {MinCloud: "v1.0.0"},
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "manifest that cannot be read",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "manifest whose min_cloud is not a version",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": installerManifest("v1.5.0", "latest"),
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "numeric ordering",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v1.9.0"}, {Version: "v1.10.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.9.0":  installerManifest("v1.9.0", "v1.0.0"),
				"v1.10.0": installerManifest("v1.10.0", "v1.0.0"),
			},
			want: "v1.10.0",
		},
		{
			name:     "min_cloud equal to the cloud",
			cloud:    "v1.4.0",
			releases: []dto.EdgeRelease{{Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.4.0": installerManifest("v1.4.0", "v1.4.0"),
			},
			want: "v1.4.0",
		},
		{
			name:     "a development cloud skips min_cloud",
			cloud:    "dev",
			releases: []dto.EdgeRelease{{Version: "v1.5.0"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v1.5.0": installerManifest("v1.5.0", "v9.0.0"),
				"v1.4.0": installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.5.0",
		},
		{
			name:     "a version that is not vX.Y.Z is ignored",
			cloud:    "v2.0.0",
			releases: []dto.EdgeRelease{{Version: "v2.0.0-rc1"}, {Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{
				"v2.0.0-rc1": installerManifest("v2.0.0-rc1", "v1.0.0"),
				"v1.4.0":     installerManifest("v1.4.0", "v1.0.0"),
			},
			want: "v1.4.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newInstallerDownloadService(t, tt.cloud, tt.releases, tt.manifests)

			got, err := svc.Current(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Version)
			assert.Equal(t, tt.manifests[tt.want].InstallerURL, got.URL)
		})
	}
}

func TestInstallerDownloadService_Current_StopsAtTheFirstEligibleRelease(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return([]dto.EdgeRelease{{Version: "v1.4.0"}, {Version: "v1.5.0"}}, nil)
	catalog.EXPECT().GetEdgeManifest(mock.Anything, "v1.5.0").Return(installerManifest("v1.5.0", "v1.0.0"), nil).Once()

	got, err := NewInstallerDownloadService(catalog, "v2.0.0").Current(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "v1.5.0", got.Version)
}

func TestInstallerDownloadService_Current_Unavailable(t *testing.T) {
	tests := []struct {
		name      string
		releases  []dto.EdgeRelease
		manifests map[string]*dto.EdgeManifest
	}{
		{name: "empty catalog"},
		{
			name:      "no edge release has an installer",
			releases:  []dto.EdgeRelease{{Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{"v1.4.0": {MinCloud: "v1.0.0"}},
		},
		{
			name:      "every release needs a newer cloud",
			releases:  []dto.EdgeRelease{{Version: "v1.4.0"}},
			manifests: map[string]*dto.EdgeManifest{"v1.4.0": installerManifest("v1.4.0", "v3.0.0")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newInstallerDownloadService(t, "v2.0.0", tt.releases, tt.manifests)

			got, err := svc.Current(context.Background())

			assert.Nil(t, got)
			assert.ErrorIs(t, err, domainError.ErrInstallerUnavailable)
		})
	}
}

func TestInstallerDownloadService_Current_ListFailureIsUnavailable(t *testing.T) {
	catalog := mocks.NewMockEdgeReleaseCatalog(t)
	catalog.EXPECT().ListEdgeReleases(mock.Anything).Return(nil, errors.New("github down"))

	_, err := NewInstallerDownloadService(catalog, "v2.0.0").Current(context.Background())

	assert.ErrorIs(t, err, domainError.ErrInstallerUnavailable)
}
