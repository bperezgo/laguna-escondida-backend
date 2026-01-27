import { apiRequest } from './config';
import type {
  PurchaseEntry,
  CreatePurchaseEntryRequest,
  PurchaseEntryListResponse,
} from '@/types/purchaseEntry';

export const purchaseEntriesApi = {
  /**
   * Get all purchase entries
   */
  async getAll(): Promise<PurchaseEntry[]> {
    const response = await apiRequest<PurchaseEntryListResponse>('/purchase-entries');
    return response.entries || [];
  },

  /**
   * Get a purchase entry by ID (includes items)
   */
  async getById(id: string): Promise<PurchaseEntry> {
    return apiRequest<PurchaseEntry>(`/purchase-entries/${id}`);
  },

  /**
   * Get purchase entries by supplier
   */
  async getBySupplier(supplierId: string): Promise<PurchaseEntry[]> {
    const response = await apiRequest<PurchaseEntryListResponse>(
      `/suppliers/${supplierId}/purchase-entries`
    );
    return response.entries || [];
  },

  /**
   * Create a new purchase entry
   */
  async create(entry: CreatePurchaseEntryRequest): Promise<PurchaseEntry> {
    return apiRequest<PurchaseEntry>('/purchase-entries', {
      method: 'POST',
      body: JSON.stringify(entry),
    });
  },
};
