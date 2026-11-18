package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Boundaries travel as JSON strings: Polygon and MultiPolygon nest coordinates to different
// depths, which a single inferred schema cannot express.
const boundaryDescription = "the zone as a GeoJSON Polygon or MultiPolygon geometry, as a JSON string, " +
	`e.g. {"type":"Polygon","coordinates":[[[-74.10,4.58],[-74.06,4.58],[-74.06,4.62],[-74.10,4.62],[-74.10,4.58]]]}. ` +
	"Positions are [longitude, latitude]; every ring is closed (last position = first)"

type createDeliveryZoneInput struct {
	Name     string `json:"name" jsonschema:"the zone name, e.g. Centro"`
	Boundary string `json:"boundary" jsonschema:"the zone as a GeoJSON Polygon or MultiPolygon geometry JSON string, positions [lng, lat]"`
	Active   *bool  `json:"active,omitempty" jsonschema:"whether deliveries are accepted in it (default true)"`
}

type updateDeliveryZoneInput struct {
	ID       string  `json:"id" jsonschema:"the delivery zone id (UUID)"`
	Name     *string `json:"name,omitempty" jsonschema:"new zone name"`
	Boundary *string `json:"boundary,omitempty" jsonschema:"new GeoJSON Polygon or MultiPolygon geometry JSON string, positions [lng, lat]"`
	Active   *bool   `json:"active,omitempty" jsonschema:"set false to stop deliveries there"`
}

type deliveryZoneIDInput struct {
	ID string `json:"id" jsonschema:"the delivery zone id (UUID)"`
}

type deliveryZoneBody struct {
	Name     *string         `json:"name,omitempty"`
	Boundary json.RawMessage `json:"boundary,omitempty"`
	Active   *bool           `json:"active,omitempty"`
}

func boundaryJSON(raw string) (json.RawMessage, error) {
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("boundary is not valid JSON: pass the GeoJSON geometry object as a JSON string")
	}
	return json.RawMessage(raw), nil
}

func registerDeliveryZoneTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_delivery_zones",
		Description: "List the delivery zones (cloud only), with their GeoJSON boundaries and active flags.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/delivery-zones", nil)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_delivery_zone",
		Description: "Create a delivery zone (cloud only). Deliveries are accepted only inside an active zone. Boundary: " +
			boundaryDescription + ". After creating it, verify it with the restaurant's own location.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createDeliveryZoneInput) (*mcp.CallToolResult, any, error) {
		boundary, err := boundaryJSON(in.Boundary)
		if err != nil {
			return toolError(err)
		}
		body, err := c.Post(ctx, "/api/delivery-zones", deliveryZoneBody{Name: &in.Name, Boundary: boundary, Active: in.Active})
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_delivery_zone",
		Description: "Update a delivery zone's name, boundary or active flag (cloud only). Existing orders keep their zone. Boundary: " + boundaryDescription + ".",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateDeliveryZoneInput) (*mcp.CallToolResult, any, error) {
		req := deliveryZoneBody{Name: in.Name, Active: in.Active}
		if in.Boundary != nil {
			boundary, err := boundaryJSON(*in.Boundary)
			if err != nil {
				return toolError(err)
			}
			req.Boundary = boundary
		}
		body, err := c.Put(ctx, "/api/delivery-zones/"+url.PathEscape(in.ID), req)
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_delivery_zone",
		Description: "Delete (soft) a delivery zone by id (cloud only). Existing orders keep their zone.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deliveryZoneIDInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Delete(ctx, "/api/delivery-zones/"+url.PathEscape(in.ID))
		if err != nil {
			return toolError(err)
		}
		return ok(body)
	})
}
