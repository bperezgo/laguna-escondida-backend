import { apiRequest } from './config';
import type {
  Table,
  CreateTableRequest,
  TableResponse,
  TableListResponse,
  Order,
  CreateOrderRequest,
  CreateOrderResponse,
  OrderResponse,
  OrderListResponse,
  ProductSearchResponse,
} from '@/types/order';

// Table endpoints
export async function getTables(): Promise<TableListResponse> {
  return apiRequest<TableListResponse>('/tables');
}

export async function getTable(id: string): Promise<TableResponse> {
  return apiRequest<TableResponse>(`/tables/${id}`);
}

export async function createTable(data: CreateTableRequest): Promise<TableResponse> {
  return apiRequest<TableResponse>('/tables', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// Order endpoints
export async function createOrder(data: CreateOrderRequest): Promise<CreateOrderResponse> {
  return apiRequest<CreateOrderResponse>('/orders', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getOrder(id: string): Promise<OrderResponse> {
  return apiRequest<OrderResponse>(`/orders/${id}`);
}

export async function getOrders(): Promise<OrderListResponse> {
  return apiRequest<OrderListResponse>('/orders');
}

export async function getOrdersByTable(tableId: string): Promise<OrderListResponse> {
  return apiRequest<OrderListResponse>(`/orders?tableId=${tableId}`);
}

// Product search endpoint
export async function searchProducts(query: string): Promise<ProductSearchResponse> {
  return apiRequest<ProductSearchResponse>(`/products/search?q=${encodeURIComponent(query)}`);
}

