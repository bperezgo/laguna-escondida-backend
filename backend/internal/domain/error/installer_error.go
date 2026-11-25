package error

import "errors"

// ErrInstallerUnavailable: no edge release the cloud can sync with carries an installer, or the releases could not be read.
var ErrInstallerUnavailable = errors.New("no installer available")
