import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import type { TableResponse } from '@/types/order';

// GET /api/tables/[id] - Get a table by ID
export async function GET(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  try {
    const response = await serverApiRequest<TableResponse>(`/tables/${params.id}`);
    return NextResponse.json(response);
  } catch (error) {
    console.error('Error fetching table:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch table' },
      { status: 500 }
    );
  }
}

