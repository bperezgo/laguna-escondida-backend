package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
	orderdto "laguna-escondida/backend/internal/platform/dto/order"
)

type createOrderInput struct {
	Body dto.CreateOrderRequest `json:"body" jsonschema:"the order (open bill) to create, including its products"`
}

type orderIDInput struct {
	ID string `json:"id" jsonschema:"the order (open bill) id (UUID)"`
}

type updateOrderInput struct {
	ID   string                 `json:"id" jsonschema:"the order (open bill) id (UUID)"`
	Body dto.UpdateOrderRequest `json:"body" jsonschema:"the order fields to update"`
}

type payOrderInput struct {
	Body orderdto.PayOrderRequest `json:"body" jsonschema:"payment details: order id, payment type and customer"`
}

type orderProductStateInput struct {
	ID                string `json:"id" jsonschema:"the order (open bill) id (UUID)"`
	OpenBillProductID string `json:"open_bill_product_id" jsonschema:"the open bill product id (UUID)"`
}

func registerOrderTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_order",
		Description: "Create a new order (open bill) with products.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createOrderInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/orders", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_active_orders",
		Description: "List all active (open) orders.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/orders", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_order",
		Description: "Get an order (open bill) with its products by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/orders/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_order",
		Description: "Update an order (open bill) by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateOrderInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/orders/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_order",
		Description: "Delete an order (open bill) by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/orders/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_closed_orders_today",
		Description: "List orders closed during today's business day.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/orders/closed", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_closed_order",
		Description: "Get a closed order with its products by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/orders/closed/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pay_order",
		Description: "Pay an order. Records the payment type and customer and closes the bill.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in payOrderInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/orders/pay-order", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	// Order product state transitions: complete / uncomplete / in-progress / cancel.
	transitions := []struct {
		name   string
		action string
		desc   string
	}{
		{"complete_order_product", "complete", "Mark an order product as completed."},
		{"uncomplete_order_product", "uncomplete", "Revert an order product to not-completed."},
		{"set_order_product_in_progress", "in-progress", "Mark an order product as in progress."},
		{"cancel_order_product", "cancel", "Cancel an order product."},
	}
	for _, t := range transitions {
		action := t.action
		mcp.AddTool(s, &mcp.Tool{
			Name:        t.name,
			Description: t.desc,
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in orderProductStateInput) (*mcp.CallToolResult, any, error) {
			path := "/api/orders/" + url.PathEscape(in.ID) + "/products/" + url.PathEscape(in.OpenBillProductID) + "/" + action
			body, err := c.Patch(ctx, path, nil)
			if err != nil {
				return toolError(err)
			}
			return ok(body)
		})
	}
}
