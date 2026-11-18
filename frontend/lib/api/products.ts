import { apiRequest } from './config';
import type { Product, ProductType, CreateProductRequest, UpdateProductRequest, ProductListResponse } from '@/types/product';

export const productsApi = {
  /**
   * Get all products, optionally filtered by product type.
   * Pass one or more types (e.g. "SELLABLE" or ["SELLABLE", "BOTH"]) to
   * restrict the result to products of those types.
   */
  async getAll(productType?: ProductType | ProductType[]): Promise<Product[]> {
    let endpoint = '/products';
    if (productType) {
      const types = Array.isArray(productType) ? productType : [productType];
      endpoint += `?product_type=${encodeURIComponent(types.join(','))}`;
    }
    const response = await apiRequest<ProductListResponse>(endpoint);
    return response.products || [];
  },

  /**
   * Get a product by ID
   */
  async getById(id: string): Promise<Product> {
    return apiRequest<Product>(`/products/${id}`);
  },

  /**
   * Create a new product
   */
  async create(product: CreateProductRequest): Promise<Product> {
    return apiRequest<Product>('/products', {
      method: 'POST',
      body: JSON.stringify(product),
    });
  },

  /**
   * Update an existing product
   */
  async update(id: string, product: UpdateProductRequest): Promise<Product> {
    return apiRequest<Product>(`/products/${id}`, {
      method: 'PUT',
      body: JSON.stringify(product),
    });
  },

  /**
   * Delete a product (soft delete)
   */
  async delete(id: string): Promise<void> {
    return apiRequest<void>(`/products/${id}`, {
      method: 'DELETE',
    });
  },

  /**
   * Get all unique product categories
   */
  async getCategories(): Promise<string[]> {
    return apiRequest<string[]>('/products/categories');
  },
};

