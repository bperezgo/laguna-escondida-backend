import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import { transformProduct } from '@/lib/api/transform';
import type { UpdateProductRequest } from '@/types/product';

// GET /api/products/[id] - Get a product by ID
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = (await params);
    const product = await serverApiRequest<any>(`/products/${id}`);
    const transformed = transformProduct(product);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error fetching product:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch product' },
      { status: 500 }
    );
  }
}

// PUT /api/products/[id] - Update a product
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = (await params);
    const body: UpdateProductRequest = await request.json();
    const product = await serverApiRequest<any>(`/products/${id}`, { method: 'PUT', body: JSON.stringify(body) });
    return NextResponse.json(transformProduct(product));
  } catch (error) {
    console.error('Error updating product:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to update product' },
      { status: 500 }
    );
  }
}

// DELETE /api/products/[id] - Delete a product
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = (await params);
    return await serverApiRequest<void>(`/products/${id}`, {
      method: 'DELETE',
    });
  } catch (error) {
    console.error('Error deleting product:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to delete product' },
      { status: 500 }
    );
  }
}

