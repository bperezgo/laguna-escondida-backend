package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

// createInvoiceInput reuses the full dto.ElectronicInvoice. Its many decimal
// (money) fields infer as opaque objects in the schema; send them as decimal
// strings/numbers. Cloud mode only.
type createInvoiceInput struct {
	Body dto.ElectronicInvoice `json:"body" jsonschema:"the electronic invoice to create"`
}

type invoiceListInput struct {
	Page                   string `json:"page,omitempty" jsonschema:"optional page number"`
	PageSize               string `json:"page_size,omitempty" jsonschema:"optional page size"`
	CreatedAtStart         string `json:"created_at_start,omitempty" jsonschema:"optional created-at start (RFC3339)"`
	CreatedAtEnd           string `json:"created_at_end,omitempty" jsonschema:"optional created-at end (RFC3339)"`
	NationalIdentification string `json:"national_identification,omitempty" jsonschema:"optional customer national identification filter"`
}

type invoiceExportInput struct {
	CreatedAtStart         string `json:"created_at_start,omitempty" jsonschema:"optional created-at start (RFC3339)"`
	CreatedAtEnd           string `json:"created_at_end,omitempty" jsonschema:"optional created-at end (RFC3339)"`
	NationalIdentification string `json:"national_identification,omitempty" jsonschema:"optional customer national identification filter"`
}

func registerInvoiceTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_invoice",
		Description: "Create an electronic invoice (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createInvoiceInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/invoices", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_invoices",
		Description: "List electronic invoices with optional pagination and filters (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in invoiceListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/invoices", query(
			"page", in.Page, "page_size", in.PageSize,
			"created_at_start", in.CreatedAtStart, "created_at_end", in.CreatedAtEnd,
			"national_identification", in.NationalIdentification,
		))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "export_invoices_csv",
		Description: "Export electronic invoices as CSV with optional filters (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in invoiceExportInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/invoices/export", query(
			"created_at_start", in.CreatedAtStart, "created_at_end", in.CreatedAtEnd,
			"national_identification", in.NationalIdentification,
		))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_invoice_document_urls",
		Description: "Backfill missing document URLs for invoices (admin; requires LAGUNA_ADMIN_API_KEY).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.PostAdmin(ctx, "/api/invoices/update-missing-document-urls", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
