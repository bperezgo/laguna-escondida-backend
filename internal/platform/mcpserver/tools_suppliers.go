package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createSupplierInput struct {
	Body dto.CreateSupplierRequest `json:"body" jsonschema:"the supplier to create"`
}

type supplierIDInput struct {
	ID string `json:"id" jsonschema:"the supplier id (UUID)"`
}

type updateSupplierInput struct {
	ID   string                    `json:"id" jsonschema:"the supplier id (UUID)"`
	Body dto.UpdateSupplierRequest `json:"body" jsonschema:"the supplier fields to update"`
}

func registerSupplierTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_supplier",
		Description: "Create a new supplier.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createSupplierInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/suppliers", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_suppliers",
		Description: "List all suppliers.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/suppliers", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_supplier",
		Description: "Get a single supplier by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supplierIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/suppliers/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_supplier",
		Description: "Update a supplier by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateSupplierInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/suppliers/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_supplier",
		Description: "Delete a supplier by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supplierIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/suppliers/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
