// Table/Customer related types
export interface Table {
  ID: string;
  Number: number;
  Status: 'available' | 'occupied' | 'reserved';
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

// Order Item types
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

// Order types
export interface Order {
  ID: string;
  TableID: string;
  TableNumber: number;
  Status: 'pending' | 'confirmed' | 'preparing' | 'ready' | 'delivered' | 'cancelled';
  Items: OrderItemWithProduct[];
  TotalAmount: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
}

export interface CreateOrderRequest {
  TableID: string;
  Items: OrderItem[];
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

