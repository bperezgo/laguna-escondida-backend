import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import { transformStock, transformStockListResponse } from '@/lib/api/transform';
import type {
  CreateStockRequest,
  BulkStockCreationOrUpdatingRequest,
  StockListResponse,
} from '@/types/stock';

// GET /api/stock - Get all stocks (non-deleted)
export async function GET() {
  try {
    const response = await serverApiRequest<any>('/stock');
    const transformed = transformStockListResponse(response);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error fetching stocks:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch stocks' },
      { status: 500 }
    );
  }
}

// POST /api/stock - Create a new stock
export async function POST(request: NextRequest) {
  try {
    const body: CreateStockRequest | BulkStockCreationOrUpdatingRequest = await request.json();
    
    // Check if it's a bulk request
    if ('items' in body) {
      // Bulk stock creation or updating
      const response = await serverApiRequest<any>('/stock/bulk', {
        method: 'POST',
        body: JSON.stringify(body),
      });
      const transformed = transformStockListResponse(response);
      return NextResponse.json(transformed);
    } else {
      // Single stock creation
      const stock = await serverApiRequest<any>('/stock', {
        method: 'POST',
        body: JSON.stringify(body),
      });
      const transformed = transformStock(stock);
      return NextResponse.json(transformed);
    }
  } catch (error) {
    console.error('Error creating stock:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create stock' },
      { status: 500 }
    );
  }
}

