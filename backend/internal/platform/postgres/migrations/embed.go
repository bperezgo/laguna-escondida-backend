// Package migrations embeds the SQL migration files into the binary. Embedding
// (vs. reading them from a cwd-relative file:// path at runtime) makes the
// migrator work identically on every OS and from any working directory — the
// file:// source driver mishandles Windows drive paths (C:\...), which broke the
// native Windows edge node.
//
// shared/ is the history both binaries ran up to v71 and is frozen. Each binary
// continues from there with its own set: a change to a table both keep needs a twin
// in cloud/ and edge/ (TestEdgeCloudSharedSchemaParity enforces it).
package migrations

import (
	"embed"
	"errors"
	"io/fs"
	"sort"
)

// SharedVersion is the last migration both binaries share.
const SharedVersion = 71

//go:embed shared/*.sql
var sharedFS embed.FS

// all: so the .keep placeholder embeds while a set has no migrations yet.
//
//go:embed all:cloud
var cloudFS embed.FS

//go:embed all:edge
var edgeFS embed.FS

// Cloud and Edge are each binary's migrations, flat at ".", for golang-migrate's iofs source.
var (
	Cloud fs.FS = layered{mustSub(cloudFS, "cloud"), mustSub(sharedFS, "shared")}
	Edge  fs.FS = layered{mustSub(edgeFS, "edge"), mustSub(sharedFS, "shared")}
)

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// layered serves several directories as one; golang-migrate rejects a version present in two.
type layered []fs.FS

func (l layered) Open(name string) (fs.File, error) {
	for _, layer := range l {
		f, err := layer.Open(name)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (l layered) ReadDir(name string) ([]fs.DirEntry, error) {
	var all []fs.DirEntry
	for _, layer := range l {
		entries, err := fs.ReadDir(layer, name)
		if err != nil {
			return nil, err
		}
		all = append(all, entries...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name() < all[j].Name() })
	return all, nil
}
