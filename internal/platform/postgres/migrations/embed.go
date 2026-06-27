// Package migrations embeds the SQL migration files into the binary. Embedding
// (vs. reading them from a cwd-relative file:// path at runtime) makes the
// migrator work identically on every OS and from any working directory — the
// file:// source driver mishandles Windows drive paths (C:\...), which broke the
// native Windows edge node.
package migrations

import "embed"

// FS holds every *.up.sql / *.down.sql in this directory, consumed via
// golang-migrate's source/iofs driver in cmd/main.go.
//
//go:embed *.sql
var FS embed.FS
