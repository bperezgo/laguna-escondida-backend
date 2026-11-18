import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import type { AddOrDecreaseStockRequest } from '@/types/stock';

// POST /api/stock/add-or-decrease - Add or decrease stock amount
// Proxies to the backend's PUT /api/stock/:product_id/add-or-decrease (returns 204)
export async function POST(request: NextRequest) {
  try {
    const body: AddOrDecreaseStockRequest = await request.json();
    return await serverApiRequest<void>(
      `/stock/${body.product_id}/add-or-decrease`,
      {
        method: 'PUT',
        body: JSON.stringify(body),
      }
    );
  } catch (error) {
    console.error('Error updating stock:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to update stock' },
      { status: 500 }
    );
  }
}
