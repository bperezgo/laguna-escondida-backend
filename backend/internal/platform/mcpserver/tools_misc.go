package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type billOwnerIDInput struct {
	ID string `json:"id" jsonschema:"the bill owner id (UUID)"`
}

type areaInput struct {
	Area string `json:"area" jsonschema:"the preparation area (e.g. kitchen, bar)"`
}

func registerMiscTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "backend_health",
		Description: "Check the backend API health.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/health", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_edge_status",
		Description: "Get the edge node sync status (mode, online, sync lag, pending sync ops).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/edge/status", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_bill_owner",
		Description: "Get a bill owner by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in billOwnerIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/bill-owners/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_pending_products_by_area",
		Description: "List pending (not-yet-completed) order products for a preparation area.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in areaInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/open-bill-products/"+url.PathEscape(in.Area)+"/pending", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_completed_products_by_area",
		Description: "List completed order products for a preparation area (today's business day).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in areaInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/open-bill-products/"+url.PathEscape(in.Area)+"/completed", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
