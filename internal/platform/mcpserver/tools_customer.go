package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/dto"
)

// Set by the agent runtime from the messaging channel, never by the model: no customer tool
// takes a phone or coordinates as an argument.
const (
	customerPhoneHeader    = "X-Customer-Phone"
	customerLocationHeader = "X-Customer-Location"

	menuProductTypes      = "SELLABLE,COMPOSITE,BOTH"
	sideDishLookupWorkers = 8

	paymentAwaiting = "awaiting_payment"
	paymentPaid     = "paid"
)

// errOrderNotFound is the same for another customer's order and a paid or voided one, so the
// tool never reveals whether an id exists.
var errOrderNotFound = errors.New("order not found: it is not one of this customer's unpaid orders")

func headerValue(req *mcp.CallToolRequest, name string) string {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return ""
	}
	return strings.TrimSpace(req.Extra.Header.Get(name))
}

// customerPhone reads the channel's verified sender, always international (e.g. 573005551234).
func customerPhone(req *mcp.CallToolRequest) (string, error) {
	raw := headerValue(req, customerPhoneHeader)
	if raw == "" {
		return "", fmt.Errorf("the customer phone is missing: the channel did not send %s", customerPhoneHeader)
	}
	phone, err := shared.NewInternationalPhone(raw)
	if err != nil {
		return "", fmt.Errorf("the customer phone in %s is not a valid international number", customerPhoneHeader)
	}
	return phone.Value(), nil
}

// customerLocation reads the pin the customer shared, as "<latitude>,<longitude>".
func customerLocation(req *mcp.CallToolRequest) (lat, lng string, err error) {
	raw := headerValue(req, customerLocationHeader)
	if raw == "" {
		return "", "", errors.New("the customer's location is missing: ask the customer to share their location (pin) in the chat")
	}
	invalid := fmt.Errorf("the customer's location in %s is invalid: expected <latitude>,<longitude> in decimal degrees", customerLocationHeader)
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return "", "", invalid
	}
	lat, lng = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	latValue, latErr := strconv.ParseFloat(lat, 64)
	lngValue, lngErr := strconv.ParseFloat(lng, 64)
	if latErr != nil || lngErr != nil || latValue < -90 || latValue > 90 || lngValue < -180 || lngValue > 180 {
		return "", "", invalid
	}
	return lat, lng, nil
}

func jsonOK(data []byte, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return toolError(fmt.Errorf("encode result: %w", err))
	}
	return ok(data)
}

// backendFailure states the backend's reason without its status line or path.
func backendFailure(action string, err error) (*mcp.CallToolResult, any, error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return toolError(fmt.Errorf("%s: %s", action, apiErr.Message()))
	}
	return toolError(fmt.Errorf("%s: %w", action, err))
}

// ---------------------------------------------------------------------------
// Menu and location
// ---------------------------------------------------------------------------

type menuItem struct {
	ProductID          string          `json:"product_id"`
	Name               string          `json:"name"`
	Category           string          `json:"category"`
	Description        *string         `json:"description,omitempty"`
	Price              decimal.Decimal `json:"price"`
	HasSideDishOptions bool            `json:"has_side_dish_options"`
}

type menuProductInput struct {
	ProductID string `json:"product_id" jsonschema:"the product id (UUID) from list_menu"`
}

type sideDishChoice struct {
	IngredientProductID string `json:"ingredient_product_id"`
	Name                string `json:"name"`
	DefaultQuantity     int    `json:"default_quantity"`
	MinQuantity         int    `json:"min_quantity"`
	MaxQuantity         int    `json:"max_quantity"`
}

type locationCheck struct {
	Deliverable bool    `json:"deliverable"`
	ZoneName    *string `json:"zone_name,omitempty"`
}

func (c *Client) sideDishOptions(ctx context.Context, productID string) ([]*dto.SideDishOption, error) {
	body, err := c.Get(ctx, "/api/products/"+url.PathEscape(productID)+"/side-dishes", nil)
	if err != nil {
		return nil, err
	}
	var resp dto.SideDishOptionListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode side dishes: %w", err)
	}
	return resp.SideDishes, nil
}

func (c *Client) listMenu(ctx context.Context) ([]menuItem, error) {
	body, err := c.Get(ctx, "/api/products", query("product_type", menuProductTypes))
	if err != nil {
		return nil, err
	}
	var resp dto.ProductListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}

	menu := make([]menuItem, 0, len(resp.Products))
	var composites []int
	for _, p := range resp.Products {
		// Ingredient-only products are never for sale, whatever the backend returns.
		if p == nil || p.ProductType == dto.ProductTypeIngredient {
			continue
		}
		if p.ProductType == dto.ProductTypeComposite {
			composites = append(composites, len(menu))
		}
		menu = append(menu, menuItem{
			ProductID:   p.ID,
			Name:        p.Name,
			Category:    p.Category,
			Description: p.Description,
			Price:       p.TotalPriceWithTaxes,
		})
	}

	// Only composites can carry side-dish options; the product list does not say which do.
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(sideDishLookupWorkers)
	for _, index := range composites {
		group.Go(func() error {
			options, err := c.sideDishOptions(groupCtx, menu[index].ProductID)
			if err != nil {
				return err
			}
			menu[index].HasSideDishOptions = len(options) > 0
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return menu, nil
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

type sideDishInput struct {
	IngredientProductID string `json:"ingredient_product_id" jsonschema:"the side-dish ingredient id from get_side_dish_options"`
	Quantity            int    `json:"quantity" jsonschema:"chosen quantity, within the option's min and max (0 removes it)"`
}

type orderLineInput struct {
	ProductID  string          `json:"product_id" jsonschema:"the product id (UUID) from list_menu"`
	Quantity   int             `json:"quantity" jsonschema:"how many, at least 1"`
	Notes      *string         `json:"notes,omitempty" jsonschema:"optional preparation notes, e.g. sin cebolla"`
	SideDishes []sideDishInput `json:"side_dishes,omitempty" jsonschema:"optional side-dish choices; omitted ones use their default"`
}

type createDeliveryOrderInput struct {
	OrderType       string           `json:"order_type" jsonschema:"takeaway (the customer picks it up) or delivery (taken to the customer's shared location)"`
	Lines           []orderLineInput `json:"lines" jsonschema:"what the customer orders, at least one line"`
	CustomerName    *string          `json:"customer_name,omitempty" jsonschema:"the customer's name, if they gave it"`
	DeliveryAddress *string          `json:"delivery_address,omitempty" jsonschema:"for delivery: street, building, apartment or door, as a reference for the rider"`
}

type updateMyOrderInput struct {
	OrderID string           `json:"order_id" jsonschema:"the order id returned by create_delivery_order or get_my_orders"`
	Lines   []orderLineInput `json:"lines" jsonschema:"the full new list of lines; it replaces the current one"`
}

type orderIDOnlyInput struct {
	OrderID string `json:"order_id" jsonschema:"the order id returned by create_delivery_order or get_my_orders"`
}

type orderLineView struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Notes    *string `json:"notes,omitempty"`
	State    string  `json:"state,omitempty"`
}

type orderView struct {
	OrderID      string          `json:"order_id"`
	OrderType    dto.OrderType   `json:"order_type"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
	Total        decimal.Decimal `json:"total"`
	ZoneID       *string         `json:"zone_id,omitempty"`
	PaymentState string          `json:"payment_state"`
	Lines        []orderLineView `json:"lines"`
}

type myOrders struct {
	Orders []orderView `json:"orders"`
}

func validateLines(lines []orderLineInput) error {
	if len(lines) == 0 {
		return errors.New("an order needs at least one line")
	}
	for _, l := range lines {
		if strings.TrimSpace(l.ProductID) == "" {
			return errors.New("every line needs a product_id from list_menu")
		}
		if l.Quantity < 1 {
			return fmt.Errorf("the quantity of product %s must be at least 1", l.ProductID)
		}
	}
	return nil
}

// orderItems turns tool lines into order lines. Lines reuse the ids of existing lines of the
// same product, in order; the rest get new ids, so the model never handles UUIDs.
func orderItems(lines []orderLineInput, existing []dto.OpenBillProductDetail) []dto.OrderProductItem {
	reusable := make(map[string][]string)
	for _, e := range existing {
		reusable[e.Product.ID] = append(reusable[e.Product.ID], e.OpenBillProductID)
	}
	items := make([]dto.OrderProductItem, 0, len(lines))
	for _, l := range lines {
		lineID := uuid.Must(uuid.NewV7()).String()
		if ids := reusable[l.ProductID]; len(ids) > 0 {
			lineID, reusable[l.ProductID] = ids[0], ids[1:]
		}
		item := dto.OrderProductItem{
			OpenBillProductID: lineID,
			ProductID:         l.ProductID,
			Quantity:          l.Quantity,
			Notes:             l.Notes,
		}
		for _, sd := range l.SideDishes {
			item.SideDishes = append(item.SideDishes, dto.SideDishSelection{IngredientProductID: sd.IngredientProductID, Quantity: sd.Quantity})
		}
		items = append(items, item)
	}
	return items
}

// writtenOrderView trims a create/update response; its products are the catalog entries, so
// names come from there and quantities from the lines that were sent.
func writtenOrderView(order *dto.OpenBill, items []dto.OrderProductItem) orderView {
	names := make(map[string]string, len(order.Products))
	for _, p := range order.Products {
		names[p.ID] = p.Name
	}
	view := orderView{
		OrderID:      order.ID,
		OrderType:    order.OrderType,
		Total:        order.TotalAmount,
		ZoneID:       order.DeliveryZoneID,
		PaymentState: paymentAwaiting,
		Lines:        make([]orderLineView, 0, len(items)),
	}
	for _, item := range items {
		view.Lines = append(view.Lines, orderLineView{Name: names[item.ProductID], Quantity: item.Quantity, Notes: item.Notes})
	}
	return view
}

func detailOrderView(order *dto.OpenBillWithProducts) orderView {
	createdAt := order.CreatedAt
	view := orderView{
		OrderID:      order.ID,
		OrderType:    order.OrderType,
		CreatedAt:    &createdAt,
		Total:        order.TotalAmount,
		ZoneID:       order.DeliveryZoneID,
		PaymentState: paymentAwaiting,
		Lines:        make([]orderLineView, 0, len(order.Products)),
	}
	if order.PaidAt != nil {
		view.PaymentState = paymentPaid
	}
	for _, p := range order.Products {
		view.Lines = append(view.Lines, orderLineView{Name: p.Product.Name, Quantity: p.Quantity, Notes: p.Notes, State: string(p.Status)})
	}
	return view
}

func (c *Client) ordersOfPhone(ctx context.Context, path, phone string) ([]*dto.OpenBillWithCreator, error) {
	body, err := c.Get(ctx, path, query("customer_phone", phone))
	if err != nil {
		return nil, err
	}
	var list dto.OpenBillListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode orders: %w", err)
	}
	return list.OpenBills, nil
}

// ownsUnpaidOrder: only the active list counts, so a paid or voided order is "not found".
func (c *Client) ownsUnpaidOrder(ctx context.Context, phone, orderID string) (bool, error) {
	orders, err := c.ordersOfPhone(ctx, "/api/orders", phone)
	if err != nil {
		return false, err
	}
	for _, o := range orders {
		if o.ID == orderID {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) orderDetail(ctx context.Context, path string) (*dto.OpenBillWithProducts, error) {
	body, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	var order dto.OpenBillWithProducts
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, fmt.Errorf("decode order: %w", err)
	}
	return &order, nil
}

func (c *Client) myOrders(ctx context.Context, phone string) ([]orderView, error) {
	active, err := c.ordersOfPhone(ctx, "/api/orders", phone)
	if err != nil {
		return nil, err
	}
	closed, err := c.ordersOfPhone(ctx, "/api/orders/closed", phone)
	if err != nil {
		return nil, err
	}

	views := make([]orderView, 0, len(active)+len(closed))
	for _, o := range active {
		detail, err := c.orderDetail(ctx, "/api/orders/"+url.PathEscape(o.ID))
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			// Paid between the two reads: it is now closed.
			detail, err = c.orderDetail(ctx, "/api/orders/closed/"+url.PathEscape(o.ID))
		}
		if err != nil {
			return nil, err
		}
		views = append(views, detailOrderView(detail))
	}
	for _, o := range closed {
		// A closed order without paid_at was voided.
		if o.PaidAt == nil || containsOrder(views, o.ID) {
			continue
		}
		detail, err := c.orderDetail(ctx, "/api/orders/closed/"+url.PathEscape(o.ID))
		if err != nil {
			return nil, err
		}
		views = append(views, detailOrderView(detail))
	}

	sort.SliceStable(views, func(i, j int) bool { return views[i].CreatedAt.After(*views[j].CreatedAt) })
	return views, nil
}

func containsOrder(views []orderView, orderID string) bool {
	for _, v := range views {
		if v.OrderID == orderID {
			return true
		}
	}
	return false
}

func registerCustomerTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_menu",
		Description: "List what the customer can order: each product's id, name, category, description, price with taxes, and whether it has side-dish options (see get_side_dish_options).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		menu, err := c.listMenu(ctx)
		if err != nil {
			return backendFailure("the menu could not be loaded", err)
		}
		return jsonOK(json.Marshal(menu))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_side_dish_options",
		Description: "For one product, list its side-dish choices with the allowed quantity range and the default.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in menuProductInput) (*mcp.CallToolResult, any, error) {
		options, err := c.sideDishOptions(ctx, in.ProductID)
		if err != nil {
			return backendFailure("the side-dish options could not be loaded", err)
		}
		choices := make([]sideDishChoice, 0, len(options))
		for _, o := range options {
			choice := sideDishChoice{
				IngredientProductID: o.IngredientProductID,
				DefaultQuantity:     o.DefaultQuantity,
				MinQuantity:         o.MinQuantity,
				MaxQuantity:         o.MaxQuantity,
			}
			if o.IngredientProduct != nil {
				choice.Name = o.IngredientProduct.Name
			}
			choices = append(choices, choice)
		}
		return jsonOK(json.Marshal(choices))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_delivery_location",
		Description: "Check whether the location the customer shared in the chat is inside the delivery area, and which zone. Creates nothing. If it is not deliverable, offer takeaway.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		lat, lng, err := customerLocation(req)
		if err != nil {
			return toolError(err)
		}
		body, err := c.Get(ctx, "/api/delivery-zones/check", query("lat", lat, "lng", lng))
		if err != nil {
			return backendFailure("the location could not be checked", err)
		}
		var result dto.DeliveryZoneCheckResult
		if err := json.Unmarshal(body, &result); err != nil {
			return toolError(fmt.Errorf("decode location check: %w", err))
		}
		return jsonOK(json.Marshal(locationCheck{Deliverable: result.Deliverable, ZoneName: result.ZoneName}))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_delivery_order",
		Description: "Create a takeaway or delivery order for the customer in this chat. A delivery goes to the location the customer shared " +
			"and needs the address reference for the rider. The order is awaiting payment; the kitchen starts it once it is paid.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createDeliveryOrderInput) (*mcp.CallToolResult, any, error) {
		orderType := dto.OrderType(strings.TrimSpace(in.OrderType))
		if orderType != dto.OrderTypeTakeaway && orderType != dto.OrderTypeDelivery {
			return toolError(errors.New("order_type must be takeaway or delivery; dine-in orders are taken at the restaurant"))
		}
		if err := validateLines(in.Lines); err != nil {
			return toolError(err)
		}
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		contact := dto.OrderContactRequest{OrderType: orderType, CustomerName: in.CustomerName, CustomerPhone: &phone}
		if orderType == dto.OrderTypeDelivery {
			lat, lng, locErr := customerLocation(req)
			if locErr != nil {
				return toolError(locErr)
			}
			if in.DeliveryAddress == nil || strings.TrimSpace(*in.DeliveryAddress) == "" {
				return toolError(errors.New("a delivery needs delivery_address: ask the customer for the street, building and apartment"))
			}
			contact.DeliveryAddress, contact.DeliveryLat, contact.DeliveryLng = in.DeliveryAddress, &lat, &lng
		}

		items := orderItems(in.Lines, nil)
		body, err := c.Post(ctx, "/api/orders", dto.CreateOrderRequest{
			OpenBillID:          uuid.Must(uuid.NewV7()).String(),
			TemporalIdentifier:  uuid.Must(uuid.NewV7()).String(),
			Products:            items,
			OrderContactRequest: contact,
		})
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
			return toolError(errors.New("outside delivery zone: the shared location is not inside the delivery area; offer takeaway instead"))
		}
		if err != nil {
			return backendFailure("the order was not created", err)
		}
		var order dto.OpenBill
		if err := json.Unmarshal(body, &order); err != nil {
			return toolError(fmt.Errorf("decode order: %w", err))
		}
		return jsonOK(json.Marshal(writtenOrderView(&order, items)))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_my_order",
		Description: "Replace the lines of one of this customer's unpaid orders. The order type, contact and location do not change. Fails with order not found once the order is paid.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateMyOrderInput) (*mcp.CallToolResult, any, error) {
		if err := validateLines(in.Lines); err != nil {
			return toolError(err)
		}
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		owned, err := c.ownsUnpaidOrder(ctx, phone, in.OrderID)
		if err != nil {
			return backendFailure("the order could not be updated", err)
		}
		if !owned {
			return toolError(errOrderNotFound)
		}
		current, err := c.orderDetail(ctx, "/api/orders/"+url.PathEscape(in.OrderID))
		if err != nil {
			return backendFailure("the order could not be updated", err)
		}

		items := orderItems(in.Lines, current.Products)
		body, err := c.Put(ctx, "/api/orders/"+url.PathEscape(in.OrderID), dto.UpdateOrderRequest{Products: items})
		if err != nil {
			return backendFailure("the order was not updated", err)
		}
		var order dto.OpenBill
		if err := json.Unmarshal(body, &order); err != nil {
			return toolError(fmt.Errorf("decode order: %w", err))
		}
		return jsonOK(json.Marshal(writtenOrderView(&order, items)))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cancel_my_order",
		Description: "Cancel one of this customer's unpaid orders. Fails with order not found once the order is paid; then only staff can help.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in orderIDOnlyInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		owned, err := c.ownsUnpaidOrder(ctx, phone, in.OrderID)
		if err != nil {
			return backendFailure("the order could not be cancelled", err)
		}
		if !owned {
			return toolError(errOrderNotFound)
		}
		if _, err := c.Delete(ctx, "/api/orders/"+url.PathEscape(in.OrderID)); err != nil {
			return backendFailure("the order was not cancelled", err)
		}
		return jsonOK(json.Marshal(struct {
			OrderID   string `json:"order_id"`
			Cancelled bool   `json:"cancelled"`
		}{in.OrderID, true}))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_my_orders",
		Description: "List this customer's orders, newest first: unpaid ones and those paid today, with payment state (awaiting_payment or paid) " +
			"and each line's kitchen state (created, in_progress, completed, cancelled). Cancelled orders are not listed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		phone, err := customerPhone(req)
		if err != nil {
			return toolError(err)
		}
		orders, err := c.myOrders(ctx, phone)
		if err != nil {
			return backendFailure("the orders could not be loaded", err)
		}
		return jsonOK(json.Marshal(myOrders{Orders: orders}))
	})
}
