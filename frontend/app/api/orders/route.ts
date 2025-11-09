import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import type {
  CreateOrderRequest,
  CreateOrderResponse,
  OrderListResponse,
} from '@/types/order';

// GET /api/orders - Get all orders (optionally filtered by tableId)
export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams;
    const tableId = searchParams.get('tableId');
    
    const endpoint = tableId ? `/orders?tableId=${tableId}` : '/orders';
    const response = await serverApiRequest<OrderListResponse>(endpoint);
    return NextResponse.json(response);
  } catch (error) {
    console.error('Error fetching orders:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch orders' },
      { status: 500 }
    );
  }
}

// POST /api/orders - Create a new order
export async function POST(request: NextRequest) {
  try {
    const body: CreateOrderRequest = await request.json();
    const response = await serverApiRequest<CreateOrderResponse>('/orders', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    return NextResponse.json(response);
  } catch (error) {
    console.error('Error creating order:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create order' },
      { status: 500 }
    );
  }
}

