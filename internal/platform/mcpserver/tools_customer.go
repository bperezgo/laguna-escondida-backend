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
	"sync"
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

	// Where the conversation came from: forwarded as the created item's origin, never shown to the model.
	channelIDHeader               = "X-Channel-Id"
	conversationIDHeader          = "X-Conversation-Id"
	attributionSourceHeader       = "X-Attribution-Source"
	attributionAdIDHeader         = "X-Attribution-Ad-Id"
	attributionClickedAtHeader    = "X-Attribution-Clicked-At"
	attributionCampaignIDHeader   = "X-Attribution-Campaign-Id"
	attributionCampaignNameHeader = "X-Attribution-Campaign-Name"

	menuProductTypes  = "SELLABLE,COMPOSITE,BOTH"
	slotLookupWorkers = 8

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

// customerOrigin is the runtime's channel, conversation and ad, or nil when it sent none. The
// backend validates each field; a bad attribution value is dropped there, not here.
func customerOrigin(req *mcp.CallToolRequest) *dto.OriginRequest {
	optional := func(name string) *string {
		if value := headerValue(req, name); value != "" {
			return &value
		}
		return nil
	}
	attribution := &dto.AttributionRequest{
		Source:       optional(attributionSourceHeader),
		AdID:         optional(attributionAdIDHeader),
		ClickedAt:    optional(attributionClickedAtHeader),
		CampaignID:   optional(attributionCampaignIDHeader),
		CampaignName: optional(attributionCampaignNameHeader),
	}
	if attribution.Source == nil && attribution.AdID == nil && attribution.ClickedAt == nil &&
		attribution.CampaignID == nil && attribution.CampaignName == nil {
		attribution = nil
	}
	origin := &dto.OriginRequest{ChannelID: optional(channelIDHeader), ConversationID: optional(conversationIDHeader), Attribution: attribution}
	if origin.ChannelID == nil && origin.ConversationID == nil && origin.Attribution == nil {
		return nil
	}
	return origin
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

type slotChoiceView struct {
	ChoiceID  string `json:"choice_id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"default"`
}

type slotView struct {
	Name    string           `json:"name"`
	Choices []slotChoiceView `json:"choices"`
}

type slotOptions struct {
	Slots []slotView `json:"slots"`
}

func slotOptionsView(slots []dto.PlateOptionSlot) slotOptions {
	out := slotOptions{Slots: make([]slotView, 0, len(slots))}
	for _, s := range slots {
		view := slotView{Name: s.Name, Choices: make([]slotChoiceView, 0, len(s.Choices))}
		for _, c := range s.Choices {
			view.Choices = append(view.Choices, slotChoiceView{ChoiceID: c.ID, Name: c.Name, IsDefault: c.IsDefault})
		}
		out.Slots = append(out.Slots, view)
	}
	return out
}

type locationCheck struct {
	Deliverable bool    `json:"deliverable"`
	ZoneName    *string `json:"zone_name,omitempty"`
}

func (c *Client) optionSlots(ctx context.Context, productID string) ([]dto.PlateOptionSlot, error) {
	body, err := c.Get(ctx, "/api/products/"+url.PathEscape(productID)+"/option-slots", nil)
	if err != nil {
		return nil, err
	}
	var resp dto.PlateOptionSlotListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode option slots: %w", err)
	}
	return resp.Slots, nil
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

	// Only composites can carry option slots; the product list does not say which do.
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(slotLookupWorkers)
	for _, index := range composites {
		group.Go(func() error {
			slots, err := c.optionSlots(groupCtx, menu[index].ProductID)
			if err != nil {
				return err
			}
			menu[index].HasSideDishOptions = len(slots) > 0
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

type orderLineInput struct {
	ProductID string   `json:"product_id" jsonschema:"the product id (UUID) from list_menu"`
	Quantity  int      `json:"quantity" jsonschema:"how many, at least 1"`
	Notes     *string  `json:"notes,omitempty" jsonschema:"optional preparation notes, e.g. sin cebolla; a change the options do not allow goes here for staff to confirm"`
	Choices   []string `json:"choices,omitempty" jsonschema:"for a plate with slots: choice_id values from get_side_dish_options, at most one per slot; a slot left out is served with its default"`
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
	Name     string           `json:"name"`
	Quantity int              `json:"quantity"`
	Notes    *string          `json:"notes,omitempty"`
	Choices  []lineChoiceView `json:"choices,omitempty"`
	State    string           `json:"state,omitempty"`
}

type lineChoiceView struct {
	Slot   string `json:"slot"`
	Choice string `json:"choice"`
}

func lineChoiceViews(choices []dto.LineChoice) []lineChoiceView {
	if len(choices) == 0 {
		return nil
	}
	out := make([]lineChoiceView, len(choices))
	for i, c := range choices {
		out[i] = lineChoiceView{Slot: c.SlotName, Choice: c.ChoiceName}
	}
	return out
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
			Choices:           l.Choices,
		}
		items = append(items, item)
	}
	return items
}

// lineChoices names, per line, the choice each slot resolved to: the one sent or the slot's
// default, as the backend resolved it. The order already exists, so a failed lookup only
// leaves the names out.
func (c *Client) lineChoices(ctx context.Context, items []dto.OrderProductItem) map[string][]lineChoiceView {
	slotsByProduct := make(map[string][]dto.PlateOptionSlot)
	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(slotLookupWorkers)
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		productID := item.ProductID
		if seen[productID] {
			continue
		}
		seen[productID] = true
		group.Go(func() error {
			if slots, err := c.optionSlots(groupCtx, productID); err == nil {
				mu.Lock()
				slotsByProduct[productID] = slots
				mu.Unlock()
			}
			return nil
		})
	}
	_ = group.Wait()

	byLine := make(map[string][]lineChoiceView, len(items))
	for _, item := range items {
		slots := slotsByProduct[item.ProductID]
		if len(slots) == 0 {
			continue
		}
		chosen := make(map[string]bool, len(item.Choices))
		for _, id := range item.Choices {
			chosen[id] = true
		}
		views := make([]lineChoiceView, 0, len(slots))
		for _, slot := range slots {
			for _, choice := range slot.Choices {
				if chosen[choice.ID] || (choice.IsDefault && !slotHasChoice(slot, chosen)) {
					views = append(views, lineChoiceView{Slot: slot.Name, Choice: choice.Name})
					break
				}
			}
		}
		byLine[item.OpenBillProductID] = views
	}
	return byLine
}

func slotHasChoice(slot dto.PlateOptionSlot, chosen map[string]bool) bool {
	for _, choice := range slot.Choices {
		if chosen[choice.ID] {
			return true
		}
	}
	return false
}

// writtenOrderView trims a create/update response; its products are the catalog entries, so
// names come from there and quantities from the lines that were sent.
func writtenOrderView(order *dto.OpenBill, items []dto.OrderProductItem, choices map[string][]lineChoiceView) orderView {
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
		view.Lines = append(view.Lines, orderLineView{
			Name: names[item.ProductID], Quantity: item.Quantity, Notes: item.Notes, Choices: choices[item.OpenBillProductID],
		})
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
		view.Lines = append(view.Lines, orderLineView{
			Name: p.Product.Name, Quantity: p.Quantity, Notes: p.Notes, Choices: lineChoiceViews(p.Choices), State: string(p.Status),
		})
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

type orderingAvailability struct {
	Delivery dto.Availability `json:"delivery"`
	Takeaway dto.Availability `json:"takeaway"`
	Timezone string           `json:"timezone"`
}

// orderingUnavailable words the backend's ordering refusal for the agent, or nil for any other 422.
func orderingUnavailable(apiErr *APIError) error {
	var body struct {
		Code         string     `json:"code"`
		OrderType    string     `json:"order_type"`
		PausedReason *string    `json:"paused_reason"`
		NextOpensAt  *time.Time `json:"next_opens_at"`
	}
	// A body that doesn't decode has no code, and falls through to nil.
	_ = json.Unmarshal(apiErr.Body, &body)
	label := "takeaway"
	if body.OrderType == string(dto.OrderTypeDelivery) {
		label = "delivery"
	}
	switch body.Code {
	case "ORDERING_PAUSED":
		reason := "no reason given"
		if body.PausedReason != nil && *body.PausedReason != "" {
			reason = *body.PausedReason
		}
		return fmt.Errorf("%s orders are paused right now (%s); no order was created. Offer the other order type if "+
			"get_ordering_availability says it is available, or ask the customer to try again later", label, reason)
	case "OUTSIDE_ORDERING_HOURS":
		when := "no opening is scheduled in the next seven days"
		if body.NextOpensAt != nil {
			when = "it next opens at " + body.NextOpensAt.Format(time.RFC3339)
		}
		return fmt.Errorf("%s is closed right now: %s; no order was created. Tell the customer when it opens, "+
			"or offer the other order type if get_ordering_availability says it is available", label, when)
	}
	return nil
}

func registerCustomerTools(s *mcp.Server, c *Client) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_menu",
		Description: "List what the customer can order: each product's id, name, category, description, price with taxes, and whether it has options (slots) to choose from (see get_side_dish_options).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		menu, err := c.listMenu(ctx)
		if err != nil {
			return backendFailure("the menu could not be loaded", err)
		}
		return jsonOK(json.Marshal(menu))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_side_dish_options",
		Description: "For one product, list what the customer can choose, as {\"slots\": [...]}: each slot takes exactly one of its choices " +
			"and is served with its default unless the customer asks for a change; send the choice_id values as the line's choices. " +
			"An empty list means the product has nothing to choose. Only the listed choices can be ordered; anything else can only go " +
			"in the line's notes, for staff to confirm.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in menuProductInput) (*mcp.CallToolResult, any, error) {
		slots, err := c.optionSlots(ctx, in.ProductID)
		if err != nil {
			return backendFailure("the options could not be loaded", err)
		}
		return jsonOK(json.Marshal(slotOptionsView(slots)))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_ordering_availability",
		Description: "Whether the restaurant is taking delivery and takeaway orders right now. Call it before taking an order. " +
			"Each type reports available, and when it isn't, the reason: paused (with paused_reason) or outside_hours (with next_opens_at). " +
			"closes_at is when an open type stops taking orders. If the requested type is unavailable, offer the other one " +
			"or tell the customer when it next opens. Creates nothing.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ noInput) (*mcp.CallToolResult, any, error) {
		body, err := c.Get(ctx, "/api/organization/settings", nil)
		if err != nil {
			return backendFailure("the ordering availability could not be loaded", err)
		}
		var settings dto.OrganizationSettingsResponse
		if err := json.Unmarshal(body, &settings); err != nil {
			return toolError(fmt.Errorf("decode ordering availability: %w", err))
		}
		return jsonOK(json.Marshal(orderingAvailability{
			Delivery: settings.Availability.Delivery,
			Takeaway: settings.Availability.Takeaway,
			Timezone: settings.Timezone,
		}))
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
			"and needs the address reference for the rider. The order is awaiting payment; the kitchen starts it once it is paid. " +
			"For a plate with slots send choices only when the customer asked for a change; each line comes back " +
			"with the choice every slot resolved to. If a choice is refused, call get_side_dish_options again and ask the customer.",
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
			Origin:              customerOrigin(req),
			OrderContactRequest: contact,
		})
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity {
			if unavailable := orderingUnavailable(apiErr); unavailable != nil {
				return toolError(unavailable)
			}
			return toolError(errors.New("outside delivery zone: the shared location is not inside the delivery area; offer takeaway instead"))
		}
		if err != nil {
			return backendFailure("the order was not created", err)
		}
		var order dto.OpenBill
		if err := json.Unmarshal(body, &order); err != nil {
			return toolError(fmt.Errorf("decode order: %w", err))
		}
		return jsonOK(json.Marshal(writtenOrderView(&order, items, c.lineChoices(ctx, items))))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "update_my_order",
		Description: "Replace the lines of one of this customer's unpaid orders. The order type, contact and location do not change. Fails with order not found once the order is paid. " +
			"Lines take the same shape as in create_delivery_order, choices included; resend a plate's choices or it goes back to its defaults.",
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
		return jsonOK(json.Marshal(writtenOrderView(&order, items, c.lineChoices(ctx, items))))
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

	registerCustomerReservationTools(s, c)
}
