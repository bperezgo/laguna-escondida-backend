package playbooks

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validFile = `---
name: sample
description: A one-line description of when to use this.
---

# Sample

Body line one.

Body line two.
`

func TestParse_Valid(t *testing.T) {
	pb, err := parse("sample", []byte(validFile))
	require.NoError(t, err)

	assert.Equal(t, "sample", pb.Name)
	assert.Equal(t, "A one-line description of when to use this.", pb.Description)
	assert.Equal(t, "# Sample\n\nBody line one.\n\nBody line two.", pb.Content)
}

func TestParse_Errors(t *testing.T) {
	cases := []struct {
		name    string
		stem    string
		raw     string
		wantErr string
	}{
		{
			name:    "missing frontmatter",
			stem:    "sample",
			raw:     "# No frontmatter here\n",
			wantErr: "missing YAML frontmatter",
		},
		{
			name:    "unterminated frontmatter",
			stem:    "sample",
			raw:     "---\nname: sample\ndescription: x\n\n# body with no closing delimiter\n",
			wantErr: "missing closing '---'",
		},
		{
			name:    "mismatched name",
			stem:    "ingest-invoice",
			raw:     "---\nname: sample\ndescription: x\n---\n\nbody\n",
			wantErr: `name "sample" does not match filename stem "ingest-invoice"`,
		},
		{
			name:    "missing name",
			stem:    "sample",
			raw:     "---\ndescription: x\n---\n\nbody\n",
			wantErr: `missing "name"`,
		},
		{
			name:    "missing description",
			stem:    "sample",
			raw:     "---\nname: sample\n---\n\nbody\n",
			wantErr: `missing "description"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parse(tc.stem, []byte(tc.raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNewIndex_DuplicateName(t *testing.T) {
	_, err := newIndex([]Playbook{
		{Name: "same", Description: "x", Content: "a"},
		{Name: "same", Description: "y", Content: "b"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `duplicate playbook name "same"`)
}

func TestLoad_ValidAndSorted(t *testing.T) {
	fsys := fstest.MapFS{
		"zebra.md":  {Data: []byte("---\nname: zebra\ndescription: z\n---\n\nbody\n")},
		"alpha.md":  {Data: []byte("---\nname: alpha\ndescription: a\n---\n\nbody\n")},
		"README.md": nil, // exercised below
	}
	// A file whose name doesn't match its stem must fail the whole load.
	fsys["README.md"] = &fstest.MapFile{Data: []byte("---\nname: alpha\ndescription: a\n---\n\nbody\n")}
	_, err := load(fsys)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match filename stem")

	delete(fsys, "README.md")
	idx, err := load(fsys)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "zebra"}, idx.names)
}

func TestList_SummariesOnlyNoContentLeak(t *testing.T) {
	summaries := List()
	require.NotEmpty(t, summaries, "the embedded ingest-invoice playbook should be present")

	blob, err := json.Marshal(summaries)
	require.NoError(t, err)
	assert.NotContains(t, string(blob), `"content"`, "list must not leak playbook bodies")

	names := Names()
	assert.Contains(t, names, "ingest-invoice")
	assert.IsIncreasing(t, names, "names must be sorted")
}

func TestGet(t *testing.T) {
	t.Run("valid name round-trips", func(t *testing.T) {
		pb, ok := Get("ingest-invoice")
		require.True(t, ok)
		assert.Equal(t, "ingest-invoice", pb.Name)
		assert.NotEmpty(t, pb.Description)
		assert.Contains(t, pb.Content, "# Ingest a supplier invoice")
		assert.NotContains(t, pb.Content, "name: ingest-invoice", "frontmatter must be stripped from content")
	})

	t.Run("unknown name", func(t *testing.T) {
		_, ok := Get("ingestinvoice")
		assert.False(t, ok)
	})

	t.Run("empty name", func(t *testing.T) {
		_, ok := Get("")
		assert.False(t, ok)
	})
}
