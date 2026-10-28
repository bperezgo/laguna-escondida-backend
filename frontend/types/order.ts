import { Product } from "./product";
import { OpenBillProductStatus, OrderType } from "./commandItem";

// Table/Customer related types
export interface Table {
  ID: string;
  Number: number;
  Status: "available" | "occupied" | "reserved";
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
}

export interface CreateTableRequest {
  Number: number;
}

export interface TableResponse {
  table: Table;
}

export interface TableListResponse {
  tables: Table[];
  total?: number;
}

export interface OpenBillUser {
  id: string;
  user_name: string;
  name: string;
}

// Open Bill types (new system)
export interface OpenBill {
  id: string;
  temporal_identifier: string;
  descriptor?: string | null;
  created_by?: OpenBillUser;
  total_amount: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
  order_type?: OrderType;
  customer_name?: string | null;
  customer_phone?: string | null;
  delivery_address?: string | null;
  delivery_lat?: string | null;
  delivery_lng?: string | null;
  delivery_zone_id?: string | null;
  delivery_map_url?: string | null;
}

// A resolved side-dish choice on one order line (ids + quantity; no names).
export interface SideDishSelection {
  ingredient_product_id: string;
  quantity: number; // int, within the option's [min, max]
}

export interface OpenBillProduct {
  open_bill_product_id: string;
  product: Product;
  quantity: number;
  notes?: string | null;
  status: OpenBillProductStatus;
  // Resolved side-dish selections for this line (ids + quantities, no names).
  // Absent on non-composite lines. Join against GET /products/:id/side-dishes for names.
  side_dishes?: SideDishSelection[];
  /**
   * When this line item was created (ISO 8601). The backend records it (it's in
   * the kitchen SSE feed, `OpenBillProductFromSSE.created_at`); when GET
   * /orders/:id includes it, the order editor uses it to lock old items after a
   * grace window. Optional because responses may omit it.
   */
  created_at?: string;
  /** Name of the user who added this line item. Empty string when omitted. */
  created_by_name?: string;
}

export interface OpenBillWithProducts extends OpenBill {
  products: OpenBillProduct[];
}

export interface OpenBillListResponse {
  open_bills: OpenBill[];
  total?: number;
}

// Device ticket printing (edge node only — POST /api/device/print)
export interface PrintTicketRequest {
  open_bill_id: string;
  copies?: number;
}

export interface PrintTicketResponse {
  printed: boolean;
}

// Order Product Item for creating orders
export interface OrderProductItem {
  open_bill_product_id: string;
  product_id: string;
  quantity: number;
  notes?: string | null;
  // Optional per-line side-dish selections. Omitting (or []) keeps default behavior
  // (backend fills defaults). Only sent for composite lines with configured options.
  side_dishes?: SideDishSelection[];
}

// New Create Order Request matching backend
export interface CreateOrderRequest {
  open_bill_id: string;
  temporal_identifier: string;
  descriptor?: string | null;
  products: OrderProductItem[];
  /** Omitted for dine-in, so the box and dine-in payloads stay as they were. */
  order_type?: OrderType;
  customer_name?: string;
  customer_phone?: string;
  delivery_address?: string;
  /** Decimal degrees as strings, up to 6 decimals. */
  delivery_lat?: string;
  delivery_lng?: string;
}

// Update Order Request
export interface UpdateOrderRequest {
  temporal_identifier?: string;
  descriptor?: string | null;
  products: OrderProductItem[];
}

// Order Item types (legacy)
export interface OrderItem {
  ProductID: string;
  ProductName: string;
  ProductImage?: string;
  Quantity: number;
  UnitPrice: number;
  Comment?: string;
}

export interface OrderItemWithProduct extends OrderItem {
  Product: {
    ID: string;
    Name: string;
    Image?: string;
    Price: number;
    VAT: number;
  };
}

// Order types (legacy)
export interface Order {
  ID: string;
  TableID: string;
  TableNumber: number;
  Status:
    | "pending"
    | "confirmed"
    | "preparing"
    | "ready"
    | "delivered"
    | "cancelled";
  Items: OrderItemWithProduct[];
  TotalAmount: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
}

export interface CreateOrderResponse {
  order: Order;
  message?: string;
}

export interface OrderResponse {
  order: Order;
}

export interface OrderListResponse {
  orders: Order[];
  total?: number;
}

// Product search types
export interface ProductSearchResponse {
  products: Array<{
    ID: string;
    Name: string;
    Category: string;
    Price: number;
    VAT: number;
    Image?: string;
  }>;
  total?: number;
}
