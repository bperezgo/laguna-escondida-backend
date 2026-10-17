package error

import "errors"

var (
	ErrNoStagedUpdate        = errors.New("no update is staged")
	ErrUpdateVersionMismatch = errors.New("requested version is not the staged update")
	ErrUpdateAlreadyPending  = errors.New("an update is already requested or applying")
)
