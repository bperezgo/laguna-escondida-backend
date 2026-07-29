package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type addSupplierProductInput struct {
	SupplierID string                           `json:"supplier_id" jsonschema:"the supplier id (UUID)"`
	Body       dto.CreateSupplierCatalogRequest `json:"body" jsonschema:"the product to add to the supplier's catalog"`
}

type updateSupplierProductInput struct {
	SupplierID string                           `json:"supplier_id" jsonschema:"the supplier id (UUID)"`
	ProductID  string                           `json:"product_id" jsonschema:"the product id (UUID)"`
	Body       dto.UpdateSupplierCatalogRequest `json:"body" jsonschema:"the catalog fields to update"`
}

type supplierProductRefInput struct {
	SupplierID string `json:"supplier_id" jsonschema:"the supplier id (UUID)"`
	ProductID  string `json:"product_id" jsonschema:"the product id (UUID)"`
}

type supplierProductsListInput struct {
	SupplierID string `json:"supplier_id" jsonschema:"the supplier id (UUID)"`
}

type productSuppliersListInput struct {
	ProductID string `json:"product_id" jsonschema:"the product id (UUID)"`
}

func registerSupplierCatalogTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_supplier_product",
		Description: "Add a product to a supplier's catalog.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addSupplierProductInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/suppliers/"+url.PathEscape(in.SupplierID)+"/products", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_supplier_product",
		Description: "Update a product entry in a supplier's catalog.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateSupplierProductInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/suppliers/"+url.PathEscape(in.SupplierID)+"/products/"+url.PathEscape(in.ProductID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "remove_supplier_product",
		Description: "Remove a product from a supplier's catalog.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supplierProductRefInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/suppliers/"+url.PathEscape(in.SupplierID)+"/products/"+url.PathEscape(in.ProductID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_supplier_products",
		Description: "List the products in a supplier's catalog.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supplierProductsListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/suppliers/"+url.PathEscape(in.SupplierID)+"/products", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_product_suppliers",
		Description: "List the suppliers that carry a given product.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in productSuppliersListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/products/"+url.PathEscape(in.ProductID)+"/suppliers", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
