import { apiRequest } from './config';
import type {
  Supplier,
  CreateSupplierRequest,
  UpdateSupplierRequest,
  SupplierListResponse,
} from '@/types/supplier';

export const suppliersApi = {
  /**
   * Get all suppliers
   */
  async getAll(): Promise<Supplier[]> {
    const response = await apiRequest<SupplierListResponse>('/suppliers');
    return response.suppliers || [];
  },

  /**
   * Get a supplier by ID
   */
  async getById(id: string): Promise<Supplier> {
    return apiRequest<Supplier>(`/suppliers/${id}`);
  },

  /**
   * Create a new supplier
   */
  async create(supplier: CreateSupplierRequest): Promise<Supplier> {
    return apiRequest<Supplier>('/suppliers', {
      method: 'POST',
      body: JSON.stringify(supplier),
    });
  },

  /**
   * Update an existing supplier
   */
  async update(id: string, supplier: UpdateSupplierRequest): Promise<Supplier> {
    return apiRequest<Supplier>(`/suppliers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(supplier),
    });
  },

  /**
   * Delete a supplier (soft delete)
   */
  async delete(id: string): Promise<void> {
    return apiRequest<void>(`/suppliers/${id}`, {
      method: 'DELETE',
    });
  },
};
