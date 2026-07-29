package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type addProductIngredientInput struct {
	ProductID string                   `json:"product_id" jsonschema:"the composite product id (UUID)"`
	Body      dto.AddIngredientRequest `json:"body" jsonschema:"ingredient_product_id and quantity (decimal string)"`
}

type productIngredientsListInput struct {
	ProductID string `json:"product_id" jsonschema:"the composite product id (UUID)"`
}

type updateProductIngredientInput struct {
	ProductID    string                      `json:"product_id" jsonschema:"the composite product id (UUID)"`
	IngredientID string                      `json:"ingredient_id" jsonschema:"the ingredient product id (UUID)"`
	Body         dto.UpdateIngredientRequest `json:"body" jsonschema:"the new quantity (decimal string)"`
}

type removeProductIngredientInput struct {
	ProductID    string `json:"product_id" jsonschema:"the composite product id (UUID)"`
	IngredientID string `json:"ingredient_id" jsonschema:"the ingredient product id (UUID)"`
}

func registerProductIngredientTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_product_ingredient",
		Description: "Add an ingredient (with quantity) to a composite product.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addProductIngredientInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_product_ingredients",
		Description: "List the ingredients of a composite product.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productIngredientsListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_product_ingredient",
		Description: "Update the quantity of an ingredient in a composite product.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateProductIngredientInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients/"+url.PathEscape(in.IngredientID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "remove_product_ingredient",
		Description: "Remove an ingredient from a composite product.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in removeProductIngredientInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients/"+url.PathEscape(in.IngredientID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
