import { apiRequest } from './config';
import type { 
  ProductResponsibility, 
  CreateProductResponsibilityRequest, 
  UpdateProductResponsibilityRequest 
} from '@/types/product';

export const productResponsibilitiesApi = {
  /**
   * Get a product responsibility by ID
   */
  async getById(id: string): Promise<ProductResponsibility> {
    return apiRequest<ProductResponsibility>(`/product-responsibilities/${id}`);
  },

  /**
   * Create a new product responsibility
   */
  async create(data: CreateProductResponsibilityRequest): Promise<ProductResponsibility> {
    return apiRequest<ProductResponsibility>('/product-responsibilities', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * Update an existing product responsibility
   */
  async update(id: string, data: UpdateProductResponsibilityRequest): Promise<ProductResponsibility> {
    return apiRequest<ProductResponsibility>(`/product-responsibilities/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  },

  /**
   * Delete a product responsibility
   */
  async delete(id: string): Promise<void> {
    return apiRequest<void>(`/product-responsibilities/${id}`, {
      method: 'DELETE',
    });
  },
};
