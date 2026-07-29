package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type financialSummaryInput struct {
	StartDate string `json:"start_date" jsonschema:"start of the range, RFC3339 (e.g. 2026-07-01T00:00:00Z)"`
	EndDate   string `json:"end_date" jsonschema:"end of the range, RFC3339 (e.g. 2026-07-31T23:59:59Z)"`
}

type dailyCloseInput struct {
	Date string `json:"date,omitempty" jsonschema:"optional business day as YYYY-MM-DD; defaults to today"`
}

func registerFinancialTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_financial_summary",
		Description: "Get a financial summary (income, taxes, totals) for a date range.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in financialSummaryInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/financial/summary", query("start_date", in.StartDate, "end_date", in.EndDate))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_daily_close",
		Description: "Get the daily close reconciliation report for a business day (defaults to today).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in dailyCloseInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/reports/daily-close", query("date", in.Date))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
