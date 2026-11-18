import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import type {
  CreateTableRequest,
  TableResponse,
  TableListResponse,
} from '@/types/order';

// GET /api/tables - Get all tables
export async function GET() {
  try {
    return await serverApiRequest<TableListResponse>('/tables');
  } catch (error) {
    console.error('Error fetching tables:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch tables' },
      { status: 500 }
    );
  }
}

// POST /api/tables - Create a new table
export async function POST(request: NextRequest) {
  try {
    const body: CreateTableRequest = await request.json();
    return await serverApiRequest<TableResponse>('/tables', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error('Error creating table:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create table' },
      { status: 500 }
    );
  }
}

