package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

// createProductBody mirrors dto.CreateProductRequest but represents the
// tri-state preparation_responsibility as a plain optional object (omit for
// none), instead of the leaky Set/Value wrapper the reflection schema exposes.
type createProductBody struct {
	Name                      string                          `json:"name" jsonschema:"product name"`
	Category                  string                          `json:"category" jsonschema:"product category"`
	ProductType               string                          `json:"product_type" jsonschema:"one of SELLABLE, INGREDIENT, COMPOSITE, BOTH"`
	UnitOfMeasure             string                          `json:"unit_of_measure" jsonschema:"one of unit, kg, g, l, ml"`
	VAT                       string                          `json:"vat" jsonschema:"VAT as a numeric string, e.g. \"19\""`
	ICO                       string                          `json:"ico" jsonschema:"ICO tax as a numeric string, e.g. \"0\""`
	TaxesFormat               string                          `json:"taxes_format" jsonschema:"one of percentage, fixed"`
	Description               *string                         `json:"description,omitempty" jsonschema:"optional description"`
	SKU                       string                          `json:"sku" jsonschema:"stock keeping unit"`
	TotalPriceWithTaxes       string                          `json:"total_price_with_taxes" jsonschema:"total price incl. taxes as a decimal string, e.g. \"12.50\""`
	PreparationResponsibility *dto.ProductResponsibilityInput `json:"preparation_responsibility,omitempty" jsonschema:"optional preparation area + priority; omit for none"`
}

// updateProductBody mirrors dto.UpdateProductRequest, representing the decimal
// price as a decimal string and preparation_responsibility as a plain optional
// object (omit to leave unchanged).
type updateProductBody struct {
	Name                      string                          `json:"name" jsonschema:"product name"`
	Category                  string                          `json:"category" jsonschema:"product category"`
	ProductType               string                          `json:"product_type" jsonschema:"one of SELLABLE, INGREDIENT, COMPOSITE, BOTH"`
	UnitOfMeasure             string                          `json:"unit_of_measure" jsonschema:"one of unit, kg, g, l, ml"`
	Price                     string                          `json:"price" jsonschema:"unit price as a decimal string, e.g. \"12.50\""`
	VAT                       string                          `json:"vat" jsonschema:"VAT as a numeric string, e.g. \"19\""`
	ICO                       string                          `json:"ico" jsonschema:"ICO tax as a numeric string, e.g. \"0\""`
	TaxesFormat               string                          `json:"taxes_format" jsonschema:"one of percentage, fixed"`
	Description               *string                         `json:"description,omitempty" jsonschema:"optional description"`
	SKU                       string                          `json:"sku" jsonschema:"stock keeping unit"`
	TotalPriceWithTaxes       string                          `json:"total_price_with_taxes" jsonschema:"total price incl. taxes as a decimal string, e.g. \"12.50\""`
	PreparationResponsibility *dto.ProductResponsibilityInput `json:"preparation_responsibility,omitempty" jsonschema:"optional preparation area + priority; omit to leave unchanged"`
}

type createProductInput struct {
	Body createProductBody `json:"body" jsonschema:"the product to create"`
}

type bulkCreateProductsInput struct {
	Body dto.BulkCreateProductRequest `json:"body" jsonschema:"the products to create in bulk"`
}

type listProductsInput struct {
	ProductType string `json:"product_type,omitempty" jsonschema:"optional comma-separated product types to filter by (e.g. SELLABLE,INGREDIENT)"`
}

type productIDInput struct {
	ID string `json:"id" jsonschema:"the product id (UUID)"`
}

type updateProductInput struct {
	ID   string            `json:"id" jsonschema:"the product id (UUID)"`
	Body updateProductBody `json:"body" jsonschema:"the product fields to update"`
}

func registerProductTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_product",
		Description: "Create a new product (menu/catalog item).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createProductInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/products", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "bulk_create_products",
		Description: "Create multiple products in a single request.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in bulkCreateProductsInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/products/bulk", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_products",
		Description: "List products, optionally filtered by product_type (comma-separated).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listProductsInput) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		if in.ProductType != "" {
			q.Set("product_type", in.ProductType)
		}
		body, err := c.Get(ctx, "/api/products", q)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_product",
		Description: "Get a single product by its id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/products/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_product",
		Description: "Update an existing product by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateProductInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/products/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_product",
		Description: "Delete a product by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/products/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_product_categories",
		Description: "List all product categories.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/products/categories", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
