package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

// createSupportDocumentInput reuses the full dto.SupportDocument. Its decimal
// (money) fields infer as opaque objects in the schema; send them as decimal
// strings/numbers. Cloud mode only.
type createSupportDocumentInput struct {
	Body dto.SupportDocument `json:"body" jsonschema:"the support document to create"`
}

type supportDocumentListInput struct {
	Page                   string `json:"page,omitempty" jsonschema:"optional page number"`
	PageSize               string `json:"page_size,omitempty" jsonschema:"optional page size"`
	CreatedAtStart         string `json:"created_at_start,omitempty" jsonschema:"optional created-at start (RFC3339)"`
	CreatedAtEnd           string `json:"created_at_end,omitempty" jsonschema:"optional created-at end (RFC3339)"`
	ProviderDocumentNumber string `json:"provider_document_number,omitempty" jsonschema:"optional provider document number filter"`
}

type supportDocumentExportInput struct {
	CreatedAtStart         string `json:"created_at_start,omitempty" jsonschema:"optional created-at start (RFC3339)"`
	CreatedAtEnd           string `json:"created_at_end,omitempty" jsonschema:"optional created-at end (RFC3339)"`
	ProviderDocumentNumber string `json:"provider_document_number,omitempty" jsonschema:"optional provider document number filter"`
}

func registerSupportDocumentTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_support_document",
		Description: "Create a support document (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createSupportDocumentInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/support-documents", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_support_documents",
		Description: "List support documents with optional pagination and filters (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supportDocumentListInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/support-documents", query(
			"page", in.Page, "page_size", in.PageSize,
			"created_at_start", in.CreatedAtStart, "created_at_end", in.CreatedAtEnd,
			"provider_document_number", in.ProviderDocumentNumber,
		))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "export_support_documents_csv",
		Description: "Export support documents as CSV with optional filters (cloud mode only).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in supportDocumentExportInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/support-documents/export", query(
			"created_at_start", in.CreatedAtStart, "created_at_end", in.CreatedAtEnd,
			"provider_document_number", in.ProviderDocumentNumber,
		))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_support_document_urls",
		Description: "Backfill missing document URLs for support documents (admin; requires LAGUNA_ADMIN_API_KEY).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.PostAdmin(ctx, "/api/support-documents/update-missing-document-urls", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
