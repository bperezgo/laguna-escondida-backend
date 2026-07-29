// Package mcpserver exposes the Laguna Escondida backend HTTP API as Model
// Context Protocol (MCP) tools. It is a primary adapter: it speaks MCP to
// clients (e.g. Claude Code) and calls the backend over HTTP as a client, so a
// single MCP server can target any environment via LAGUNA_API_URL. Each tool
// wraps one backend endpoint and reuses the backend's own request DTOs as its
// input schema (with tailored inputs for the few fields whose custom JSON types
// do not reflect cleanly, e.g. decimals and tri-state optionals).
package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

const serverVersion = "0.1.0"

// noInput is the input type for tools that take no arguments.
type noInput struct{}

// NewMCPServer builds the MCP server and registers every tool that wraps a
// backend endpoint.
func NewMCPServer(c *Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "laguna-escondida",
		Version: serverVersion,
	}, nil)

	registerProductTools(s, c)
	registerProductResponsibilityTools(s, c)
	registerProductIngredientTools(s, c)
	registerOrderTools(s, c)
	registerStockTools(s, c)
	registerSupplierTools(s, c)
	registerSupplierCatalogTools(s, c)
	registerPurchaseEntryTools(s, c)
	registerExpenseTools(s, c)
	registerFinancialTools(s, c)
	registerInvoiceTools(s, c)
	registerSupportDocumentTools(s, c)
	registerUserTools(s, c)
	registerMiscTools(s, c)

	return s
}

// ok wraps a successful backend response body as a tool result. A 204 (empty
// body) is reported as a short confirmation so the model always gets non-empty
// output.
func ok(body []byte) (*mcp.CallToolResult, any, error) {
	text := string(body)
	if len(text) == 0 {
		text = `{"status":"ok"}`
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

// toolError reports a backend/transport error as a tool error result (IsError)
// rather than a protocol-level error, so the model can read and react to it.
func toolError(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}, nil, nil
}
