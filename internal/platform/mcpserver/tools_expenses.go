package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createExpenseCategoryInput struct {
	Body dto.CreateExpenseCategoryRequest `json:"body" jsonschema:"the expense category to create (code + name)"`
}

type expenseCategoryIDInput struct {
	ID string `json:"id" jsonschema:"the expense category id (UUID)"`
}

type updateExpenseCategoryInput struct {
	ID   string                           `json:"id" jsonschema:"the expense category id (UUID)"`
	Body dto.UpdateExpenseCategoryRequest `json:"body" jsonschema:"the category fields to update"`
}

type createExpenseInput struct {
	Body dto.CreateExpenseRequest `json:"body" jsonschema:"the expense to create; amount is a decimal string, e.g. \"12.50\""`
}

type expenseFilterInput struct {
	CategoryID string `json:"category_id,omitempty" jsonschema:"optional expense category id (UUID) filter"`
	SupplierID string `json:"supplier_id,omitempty" jsonschema:"optional supplier id (UUID) filter"`
	StartDate  string `json:"start_date,omitempty" jsonschema:"optional start date (YYYY-MM-DD or RFC3339)"`
	EndDate    string `json:"end_date,omitempty" jsonschema:"optional end date (YYYY-MM-DD or RFC3339)"`
}

type expenseIDInput struct {
	ID string `json:"id" jsonschema:"the expense id (UUID)"`
}

type updateExpenseInput struct {
	ID   string                   `json:"id" jsonschema:"the expense id (UUID)"`
	Body dto.UpdateExpenseRequest `json:"body" jsonschema:"the expense fields to update; amount is a decimal string"`
}

type uploadExpenseDocumentInput struct {
	ID           string `json:"id" jsonschema:"the expense id (UUID)"`
	FilePath     string `json:"file_path" jsonschema:"path on the MCP server host to the document (PDF/XML/ZIP) to upload"`
	CategoryCode string `json:"category_code" jsonschema:"the expense category code the document belongs to"`
	FileType     string `json:"file_type,omitempty" jsonschema:"optional file type hint (e.g. pdf, xml, zip)"`
}

func registerExpenseTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_expense_category",
		Description: "Create an expense category.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createExpenseCategoryInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/expense-categories", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_expense_categories",
		Description: "List all expense categories.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/expense-categories", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_expense_category",
		Description: "Get an expense category by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in expenseCategoryIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/expense-categories/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_expense_category",
		Description: "Update an expense category by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateExpenseCategoryInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/expense-categories/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_expense",
		Description: "Create an expense.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createExpenseInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/expenses", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_expenses",
		Description: "List expenses, optionally filtered by category, supplier and date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in expenseFilterInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/expenses", query("category_id", in.CategoryID, "supplier_id", in.SupplierID, "start_date", in.StartDate, "end_date", in.EndDate))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_expense",
		Description: "Get an expense by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in expenseIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/expenses/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_expense",
		Description: "Update an expense by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateExpenseInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/expenses/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_expense",
		Description: "Delete an expense by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in expenseIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/expenses/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "upload_expense_document",
		Description: "Upload a document (PDF/XML/ZIP) for an expense from a file on the MCP server host.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in uploadExpenseDocumentInput) (*mcp.CallToolResult, any, error) {
		body, err := c.UploadFile(ctx, "/api/expenses/"+url.PathEscape(in.ID)+"/documents", query("category_code", in.CategoryCode, "file_type", in.FileType), "file", in.FilePath)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "export_expenses_csv",
		Description: "Export expenses as CSV, optionally filtered by category, supplier and date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in expenseFilterInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/expenses/export", query("category_id", in.CategoryID, "supplier_id", in.SupplierID, "start_date", in.StartDate, "end_date", in.EndDate))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
