package edgeupdate

import "path/filepath"

// StateDirFor is the appliance's state folder for a binary run from <root>\artifacts\edge-node\:
// the scripts fix it at <root>\state, so the box needs no setting to find it.
func StateDirFor(executable string) string {
	return filepath.Join(filepath.Dir(executable), "..", "..", "state")
}
