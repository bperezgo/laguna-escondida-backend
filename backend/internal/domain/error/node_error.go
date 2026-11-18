package error

import "errors"

// ErrNodeNotFound means a node presented the sync key with an id the cloud has not registered.
var ErrNodeNotFound = errors.New("node not registered")
