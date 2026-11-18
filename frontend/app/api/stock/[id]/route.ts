import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';

// DELETE /api/stock/[id] - Delete a stock (soft delete)
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    return await serverApiRequest<void>(`/stock/${id}`, {
      method: 'DELETE',
    });
  } catch (error) {
    console.error('Error deleting stock:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to delete stock' },
      { status: 500 }
    );
  }
}

