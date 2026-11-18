package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createPurchaseEntryInput struct {
	Body dto.CreatePurchaseEntryRequest `json:"body" jsonschema:"the purchase entry to create, including its items"`
}

type purchaseEntryFilterInput struct {
	SupplierID string `json:"supplier_id,omitempty" jsonschema:"optional supplier id (UUID) filter"`
	StartDate  string `json:"start_date,omitempty" jsonschema:"optional start date (YYYY-MM-DD or RFC3339)"`
	EndDate    string `json:"end_date,omitempty" jsonschema:"optional end date (YYYY-MM-DD or RFC3339)"`
}

type purchaseEntryIDInput struct {
	ID string `json:"id" jsonschema:"the purchase entry id (UUID)"`
}

type supplierPurchaseEntriesInput struct {
	SupplierID string `json:"supplier_id" jsonschema:"the supplier id (UUID)"`
}

type uploadPurchaseEntryDocumentInput struct {
	ID       string `json:"id" jsonschema:"the purchase entry id (UUID)"`
	FilePath string `json:"file_path" jsonschema:"path on the MCP server host to the document (PDF/XML/ZIP) to upload"`
	FileType string `json:"file_type,omitempty" jsonschema:"optional file type hint (e.g. pdf, xml, zip)"`
}

func registerPurchaseEntryTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_purchase_entry",
		Description: "Create a purchase entry (supplier invoice) with line items.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createPurchaseEntryInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/purchase-entries", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_purchase_entries",
		Description: "List purchase entries, optionally filtered by supplier and date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in purchaseEntryFilterInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/purchase-entries", query("supplier_id", in.SupplierID, "start_date", in.StartDate, "end_date", in.EndDate))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_purchase_entry",
		Description: "Get a purchase entry by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in purchaseEntryIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/purchase-entries/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_supplier_purchase_entries",
		Description: "List all purchase entries for a given supplier.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supplierPurchaseEntriesInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/suppliers/"+url.PathEscape(in.SupplierID)+"/purchase-entries", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "upload_purchase_entry_document",
		Description: "Upload a document (PDF/XML/ZIP) for a purchase entry from a file on the MCP server host.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in uploadPurchaseEntryDocumentInput) (*mcp.CallToolResult, any, error) {
		body, err := c.UploadFile(ctx, "/api/purchase-entries/"+url.PathEscape(in.ID)+"/documents", query("file_type", in.FileType), "file", in.FilePath)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "export_purchase_entries_csv",
		Description: "Export purchase entries as CSV, optionally filtered by supplier and date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in purchaseEntryFilterInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/purchase-entries/export", query("supplier_id", in.SupplierID, "start_date", in.StartDate, "end_date", in.EndDate))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
