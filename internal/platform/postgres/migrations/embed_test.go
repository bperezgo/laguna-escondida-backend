package migrations

import (
	"io/fs"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSets_ContinueTheSharedHistory(t *testing.T) {
	for name, set := range map[string]fs.FS{"cloud": Cloud, "edge": Edge} {
		t.Run(name, func(t *testing.T) {
			source, err := iofs.New(set, ".")
			require.NoError(t, err)
			defer func() { _ = source.Close() }()

			version, err := source.First()
			require.NoError(t, err)
			assert.Equal(t, uint(1), version)
			for version < SharedVersion {
				next, nextErr := source.Next(version)
				require.NoError(t, nextErr, "gap after %d", version)
				require.Equal(t, version+1, next)
				version = next
			}
			_, _, err = source.ReadUp(SharedVersion)
			require.NoError(t, err)
		})
	}
}
