package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createProductResponsibilityInput struct {
	Body dto.CreateProductResponsibilityRequest `json:"body" jsonschema:"product_name, area and priority"`
}

type productResponsibilityIDInput struct {
	ID string `json:"id" jsonschema:"the product responsibility id (UUID)"`
}

type updateProductResponsibilityInput struct {
	ID   string                                 `json:"id" jsonschema:"the product responsibility id (UUID)"`
	Body dto.UpdateProductResponsibilityRequest `json:"body" jsonschema:"the area and/or priority to update"`
}

func registerProductResponsibilityTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_product_responsibility",
		Description: "Create a preparation responsibility (area + priority) for a product by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createProductResponsibilityInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/product-responsibilities", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_product_responsibility",
		Description: "Get a product preparation responsibility by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productResponsibilityIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/product-responsibilities/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_product_responsibility",
		Description: "Update a product preparation responsibility by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateProductResponsibilityInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/product-responsibilities/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_product_responsibility",
		Description: "Delete a product preparation responsibility by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productResponsibilityIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/product-responsibilities/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
