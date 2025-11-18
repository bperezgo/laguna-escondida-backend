import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import { transformStock } from '@/lib/api/transform';
import type { AddOrDecreaseStockRequest } from '@/types/stock';

// POST /api/stock/add-or-decrease - Add or decrease stock amount
export async function POST(request: NextRequest) {
  try {
    const body: AddOrDecreaseStockRequest = await request.json();
    const stock = await serverApiRequest<any>('/stock/add-or-decrease', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const transformed = transformStock(stock);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error updating stock:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to update stock' },
      { status: 500 }
    );
  }
}

