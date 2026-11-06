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
	IngredientID string                      `json:"ingredient_id" jsonschema:"the recipe row id (UUID): the id field from list_product_ingredients, not ingredient_product_id"`
	Body         dto.UpdateIngredientRequest `json:"body" jsonschema:"the new quantity (decimal string)"`
}

type removeProductIngredientInput struct {
	ProductID    string `json:"product_id" jsonschema:"the composite product id (UUID)"`
	IngredientID string `json:"ingredient_id" jsonschema:"the recipe row id (UUID): the id field from list_product_ingredients, not ingredient_product_id"`
}

type configureSideDishInput struct {
	ProductID    string                       `json:"product_id" jsonschema:"the composite product id (UUID)"`
	IngredientID string                       `json:"ingredient_id" jsonschema:"the recipe row id (UUID): the id field from list_product_ingredients, not ingredient_product_id"`
	Body         dto.ConfigureSideDishRequest `json:"body" jsonschema:"default_quantity, min_quantity and max_quantity in whole units, with 0 <= min <= default <= max"`
}

type clearSideDishInput struct {
	ProductID    string `json:"product_id" jsonschema:"the composite product id (UUID)"`
	IngredientID string `json:"ingredient_id" jsonschema:"the recipe row id (UUID): the id field from list_product_ingredients, not ingredient_product_id"`
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

	mcp.AddTool(s, &mcp.Tool{
		Name: "configure_side_dish",
		Description: "Mark a recipe ingredient as a side dish the customer can adjust, with default/min/max in whole units " +
			"(min 0 makes it optional). The default replaces the ingredient's recipe quantity, so a side dish should be a " +
			"portion product (e.g. a composite \"Porción arroz de coco\" whose own recipe holds the grams), not a raw gram ingredient. Cloud only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in configureSideDishInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients/"+url.PathEscape(in.IngredientID)+"/side-dish", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "clear_side_dish",
		Description: "Turn a side dish back into a fixed recipe ingredient; its current default quantity stays as the fixed amount. Cloud only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in clearSideDishInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/ingredients/"+url.PathEscape(in.IngredientID)+"/side-dish")
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
