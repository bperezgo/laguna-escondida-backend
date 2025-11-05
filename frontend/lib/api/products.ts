import { apiRequest } from './config';
import type { Product, CreateProductRequest, UpdateProductRequest, ProductListResponse } from '@/types/product';

export const productsApi = {
  /**
   * Get all products
   */
  async getAll(): Promise<Product[]> {
    const response = await apiRequest<ProductListResponse>('/products');
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
};

