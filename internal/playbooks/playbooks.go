// Package playbooks serves versioned, domain-specific procedures ("playbooks")
// that describe how to correctly perform a multi-step task using the
// laguna-escondida MCP server's tools — trigger conditions, ordered steps with
// example request shapes, and gotchas learned in production.
//
// Each playbook is a single Markdown file with YAML frontmatter (name +
// description), embedded into the binary at build time and validated at startup
// so a bad edit fails fast instead of surfacing as a silent runtime gap.
package playbooks

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Playbook is one procedure: frontmatter metadata plus its Markdown body.
type Playbook struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// Summary is the catalog entry for a playbook: name + description only, so
// listing the catalog stays cheap and never leaks full bodies.
type Summary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

//go:embed *.md
var files embed.FS

// registry is loaded once at package init from the embedded files; a malformed
// or inconsistent playbook set panics here, at process startup.
var registry = mustLoad(files)

type index struct {
	byName map[string]Playbook
	names  []string // sorted, for stable listing
}

func mustLoad(fsys fs.FS) *index {
	idx, err := load(fsys)
	if err != nil {
		panic("playbooks: " + err.Error())
	}
	return idx
}

func load(fsys fs.FS) (*index, error) {
	paths, err := fs.Glob(fsys, "*.md")
	if err != nil {
		return nil, err
	}
	pbs := make([]Playbook, 0, len(paths))
	for _, p := range paths {
		raw, readErr := fs.ReadFile(fsys, p)
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", p, readErr)
		}
		stem := strings.TrimSuffix(path.Base(p), ".md")
		pb, parseErr := parse(stem, raw)
		if parseErr != nil {
			return nil, fmt.Errorf("%s: %w", p, parseErr)
		}
		pbs = append(pbs, pb)
	}
	return newIndex(pbs)
}

// newIndex builds the name→playbook lookup. Enforcing name==stem in parse makes
// a duplicate name structurally impossible for distinct files, but the guard is
// kept as defensive insurance against a future loader that relaxes that rule.
func newIndex(pbs []Playbook) (*index, error) {
	byName := make(map[string]Playbook, len(pbs))
	for _, pb := range pbs {
		if _, dup := byName[pb.Name]; dup {
			return nil, fmt.Errorf("duplicate playbook name %q", pb.Name)
		}
		byName[pb.Name] = pb
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return &index{byName: byName, names: names}, nil
}

// parse splits a playbook file into frontmatter + body and validates that the
// declared name matches the filename stem. A minimal manual split on the '---'
// delimiter lines is deliberate — two fields don't warrant a YAML dependency.
func parse(stem string, raw []byte) (Playbook, error) {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Playbook{}, errors.New("missing YAML frontmatter (file must start with a '---' line)")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return Playbook{}, errors.New("malformed YAML frontmatter (missing closing '---' line)")
	}

	var name, desc string
	for _, line := range lines[1:end] {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = unquote(strings.TrimSpace(val))
		case "description":
			desc = unquote(strings.TrimSpace(val))
		}
	}
	if name == "" {
		return Playbook{}, errors.New(`frontmatter is missing "name"`)
	}
	if desc == "" {
		return Playbook{}, errors.New(`frontmatter is missing "description"`)
	}
	if name != stem {
		return Playbook{}, fmt.Errorf("frontmatter name %q does not match filename stem %q", name, stem)
	}

	body := strings.Trim(strings.Join(lines[end+1:], "\n"), "\n")
	return Playbook{Name: name, Description: desc, Content: body}, nil
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// List returns every playbook's summary (name + description), sorted by name.
func List() []Summary {
	out := make([]Summary, 0, len(registry.names))
	for _, n := range registry.names {
		pb := registry.byName[n]
		out = append(out, Summary{Name: pb.Name, Description: pb.Description})
	}
	return out
}

// Get returns a playbook by name, and whether it exists.
func Get(name string) (Playbook, bool) {
	pb, ok := registry.byName[name]
	return pb, ok
}

// Names returns the available playbook names, sorted — handy for building a
// helpful "did you mean" error when a lookup misses.
func Names() []string {
	return append([]string(nil), registry.names...)
}
