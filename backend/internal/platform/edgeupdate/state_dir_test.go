package edgeupdate

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateDirFor_IsTheStateFolderAtTheInstallRoot(t *testing.T) {
	cases := map[string]string{
		filepath.Join("laguna-edge", "artifacts", "edge-node", "edge-node.exe"): filepath.Join("laguna-edge", "state"),
		filepath.Join("kuenta", "artifacts", "edge-node", "edge-node.exe"):      filepath.Join("kuenta", "state"),
	}
	for executable, want := range cases {
		t.Run(executable, func(t *testing.T) {
			assert.Equal(t, want, StateDirFor(executable))
		})
	}
}

func TestStateDirFor_WindowsInstallRoots(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive-letter paths only parse on Windows")
	}
	assert.Equal(t, `C:\laguna-edge\state`, StateDirFor(`C:\laguna-edge\artifacts\edge-node\edge-node.exe`))
	assert.Equal(t, `D:\kuenta\state`, StateDirFor(`D:\kuenta\artifacts\edge-node\edge-node.exe`))
}
