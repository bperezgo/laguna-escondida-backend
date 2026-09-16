package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connectPlaybookServer wires the playbook tools onto an in-memory MCP session
// so the tests exercise them through the real protocol, not the handlers directly.
func connectPlaybookServer(t *testing.T) (*mcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()

	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	registerPlaybookTools(s)

	serverT, clientT := mcp.NewInMemoryTransports()
	_, err := s.Connect(ctx, serverT, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "test"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	return cs, ctx
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, res.Content, 1)
	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected text content")
	return tc.Text
}

func TestTool_ListPlaybooks(t *testing.T) {
	cs, ctx := connectPlaybookServer(t)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_playbooks"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	text := toolText(t, res)
	assert.NotContains(t, text, `"content"`, "list must not leak playbook bodies")

	var summaries []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &summaries))
	require.NotEmpty(t, summaries)

	var names []string
	for _, s := range summaries {
		names = append(names, s.Name)
		assert.NotEmpty(t, s.Description)
	}
	assert.Contains(t, names, "ingest-invoice")
}

func TestTool_GetPlaybook(t *testing.T) {
	cs, ctx := connectPlaybookServer(t)

	t.Run("valid name returns full content", func(t *testing.T) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_playbook",
			Arguments: map[string]any{"name": "ingest-invoice"},
		})
		require.NoError(t, err)
		require.False(t, res.IsError)

		var pb struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
		}
		require.NoError(t, json.Unmarshal([]byte(toolText(t, res)), &pb))
		assert.Equal(t, "ingest-invoice", pb.Name)
		assert.NotEmpty(t, pb.Description)
		assert.Contains(t, pb.Content, "# Ingest a supplier invoice")
		assert.NotContains(t, pb.Content, "name: ingest-invoice", "frontmatter must be stripped")
	})

	t.Run("unknown name errors and lists available", func(t *testing.T) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_playbook",
			Arguments: map[string]any{"name": "ingestinvoice"},
		})
		require.NoError(t, err)
		require.True(t, res.IsError)

		text := toolText(t, res)
		assert.Contains(t, text, `unknown playbook "ingestinvoice"`)
		assert.Contains(t, text, "ingest-invoice")
	})

	t.Run("empty name errors", func(t *testing.T) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_playbook",
			Arguments: map[string]any{"name": ""},
		})
		require.NoError(t, err)
		require.True(t, res.IsError)
	})
}
