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

type setPlateOptionSlotsInput struct {
	ProductID string                             `json:"product_id" jsonschema:"the composite product id (UUID)"`
	Body      dto.ReplacePlateOptionSlotsRequest `json:"body" jsonschema:"the plate's full ordered slot list; it replaces the current one"`
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
		Name: "get_plate_option_slots",
		Description: "List a composite plate's option slots: each slot's choices in order, the default of each, and the ingredient " +
			"quantities each choice consumes. An empty list means the plate has nothing to choose.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productIngredientsListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/option-slots", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "set_plate_option_slots",
		Description: "Replace a composite plate's option slots with the full list sent. Each slot needs a name and exactly one default " +
			"choice; each choice is a bundle of ingredient products and whole quantities (an empty bundle means \"without\", e.g. Sin ensalada). " +
			"Resend existing ids (from get_plate_option_slots) to keep them; anything left out is removed. An empty list removes every slot. " +
			"Keep a choosable part out of the plate's recipe: the recipe is always consumed, the chosen bundles on top. Choices never change the price. Cloud only.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setPlateOptionSlotsInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/option-slots", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
