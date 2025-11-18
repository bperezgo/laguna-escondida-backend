import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import { transformStockListResponse } from '@/lib/api/transform';
import type { BulkStockCreationOrUpdatingRequest } from '@/types/stock';

// POST /api/stock/bulk - Bulk create or update stocks
export async function POST(request: NextRequest) {
  try {
    const body: BulkStockCreationOrUpdatingRequest = await request.json();
    const response = await serverApiRequest<any>('/stock/bulk', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const transformed = transformStockListResponse(response);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error bulk updating stocks:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to bulk update stocks' },
      { status: 500 }
    );
  }
}

