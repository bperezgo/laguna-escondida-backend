import { Product } from "./product";
import { OpenBillProductStatus } from "./commandItem";

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
}

export interface OpenBillProduct {
  open_bill_product_id: string;
  product: Product;
  quantity: number;
  notes?: string | null;
  status: OpenBillProductStatus;
}

export interface OpenBillWithProducts extends OpenBill {
  products: OpenBillProduct[];
}

export interface OpenBillListResponse {
  open_bills: OpenBill[];
  total?: number;
}

// Order Product Item for creating orders
export interface OrderProductItem {
  open_bill_product_id: string;
  product_id: string;
  quantity: number;
  notes?: string | null;
}

// New Create Order Request matching backend
export interface CreateOrderRequest {
  open_bill_id: string;
  temporal_identifier: string;
  descriptor?: string | null;
  products: OrderProductItem[];
}

// Update Order Request
export interface UpdateOrderRequest {
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
