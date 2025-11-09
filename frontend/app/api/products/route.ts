import { NextRequest, NextResponse } from 'next/server';
import { serverApiRequest } from '@/lib/api/server';
import { transformProduct, transformProductListResponse } from '@/lib/api/transform';
import type { Product, CreateProductRequest, UpdateProductRequest, ProductListResponse } from '@/types/product';

// GET /api/products - Get all products
export async function GET() {
  try {
    const response = await serverApiRequest<any>('/products');
    const transformed = transformProductListResponse(response);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error fetching products:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to fetch products' },
      { status: 500 }
    );
  }
}

// POST /api/products - Create a new product
export async function POST(request: NextRequest) {
  try {
    const body: CreateProductRequest = await request.json();
    const product = await serverApiRequest<any>('/products', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    const transformed = transformProduct(product);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error('Error creating product:', error);
    return NextResponse.json(
      { error: error instanceof Error ? error.message : 'Failed to create product' },
      { status: 500 }
    );
  }
}

