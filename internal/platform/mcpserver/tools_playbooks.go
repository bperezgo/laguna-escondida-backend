package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/playbooks"
)

type getPlaybookInput struct {
	Name string `json:"name" jsonschema:"the playbook name — its filename stem, e.g. ingest-invoice (get the valid names from list_playbooks)"`
}

// registerPlaybookTools registers read-only discovery of the server's bundled
// playbooks. Unlike the endpoint-wrapping tools these need no backend client:
// playbook content is static, embedded in the binary.
func registerPlaybookTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_playbooks",
		Description: "List the available playbooks (name + description only). A playbook is a versioned, step-by-step procedure for correctly performing a multi-step task with this server's tools (e.g. ingesting a supplier invoice) — trigger conditions, ordered steps with example request shapes, and gotchas. Call get_playbook to read one in full before starting such a task.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := json.Marshal(playbooks.List())
		if err != nil {
			return toolError(fmt.Errorf("marshal playbooks: %w", err))
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_playbook",
		Description: "Get the full content of one playbook by name: its description and Markdown body (ordered steps, example request bodies, and gotchas). Use list_playbooks to discover valid names.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in getPlaybookInput) (*mcp.CallToolResult, any, error) {
		pb, found := playbooks.Get(in.Name)
		if !found {
			return toolError(fmt.Errorf("unknown playbook %q; available: %s", in.Name, strings.Join(playbooks.Names(), ", ")))
		}
		body, err := json.Marshal(pb)
		if err != nil {
			return toolError(fmt.Errorf("marshal playbook: %w", err))
		}
		return ok(body)
	})
}
