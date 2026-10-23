// Package buildinfo holds values stamped into the binary at link time, so a running node
// can answer "what are you running?" (edge appliances report it on /api/edge/status).
//
// Stamp with:
//
//	go build -ldflags "-X laguna-escondida/backend/internal/platform/buildinfo.Version=v1.2.3" ./cmd/edge
//
// An unstamped build (go run, tests, local builds) reports "dev".
package buildinfo

// Version is the release tag this binary was built from.
var Version = "dev"
