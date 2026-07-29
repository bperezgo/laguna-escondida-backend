package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createStockInput struct {
	Body dto.CreateStockRequest `json:"body" jsonschema:"product_id (UUID) and integer amount"`
}

type adjustStockInput struct {
	Body dto.AddOrDecreaseStockRequest `json:"body" jsonschema:"product_id (UUID) and signed integer change (positive to add, negative to decrease)"`
}

type deleteStockInput struct {
	ProductID string `json:"product_id" jsonschema:"the product id (UUID)"`
}

type bulkStockInput struct {
	Body dto.BulkStockCreationOrUpdatingRequest `json:"body" jsonschema:"items to create or update (product_id + amount)"`
}

func registerStockTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_stock",
		Description: "List stock levels for all products.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/stock", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_stock",
		Description: "Create a stock record for a product (edge mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createStockInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/stock", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "adjust_stock",
		Description: "Add to or decrease a product's stock by a signed amount (edge mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in adjustStockInput) (*mcp.CallToolResult, any, error) {
		path := "/api/stock/" + url.PathEscape(in.Body.ProductID) + "/add-or-decrease"
		body, err := c.Put(ctx, path, in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_stock",
		Description: "Delete a product's stock record (edge mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteStockInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/stock/"+url.PathEscape(in.ProductID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "bulk_upsert_stock",
		Description: "Create or update stock for multiple products in one request (edge mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in bulkStockInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/stock/bulk", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
