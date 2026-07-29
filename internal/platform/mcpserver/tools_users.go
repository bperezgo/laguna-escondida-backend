package mcpserver

import (
	"context"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"laguna-escondida/backend/internal/domain/dto"
)

type createUserInput struct {
	Body dto.CreateUserRequest `json:"body" jsonschema:"the user to create (username, name, password, role_ids)"`
}

type userIDInput struct {
	ID string `json:"id" jsonschema:"the user id (UUID)"`
}

type updateUserInput struct {
	ID   string                `json:"id" jsonschema:"the user id (UUID)"`
	Body dto.UpdateUserRequest `json:"body" jsonschema:"the user fields to update"`
}

type resetUserPasswordInput struct {
	ID   string                   `json:"id" jsonschema:"the user id (UUID)"`
	Body dto.ResetPasswordRequest `json:"body" jsonschema:"the new password"`
}

func registerUserTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_user",
		Description: "Create a new user with roles.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createUserInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/admin/users", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_users",
		Description: "List all users with their roles.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/admin/users", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_user",
		Description: "Get a user (with roles) by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in userIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/admin/users/"+url.PathEscape(in.ID), nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_user",
		Description: "Update a user's name, roles or active flag by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateUserInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Put(ctx, "/api/admin/users/"+url.PathEscape(in.ID), in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "reset_user_password",
		Description: "Reset a user's password by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in resetUserPasswordInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Post(ctx, "/api/admin/users/"+url.PathEscape(in.ID)+"/reset-password", in.Body)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_user",
		Description: "Delete a user by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in userIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/admin/users/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_roles",
		Description: "List all roles.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/admin/roles", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_current_user",
		Description: "Get the user the MCP server is authenticated as (its roles and permissions).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/auth/me", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
