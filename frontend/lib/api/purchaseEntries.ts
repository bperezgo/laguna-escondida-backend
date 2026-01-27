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

  /**
   * Upload a document (PDF or XML) for a purchase entry
   */
  async uploadDocument(
    entryId: string,
    fileType: 'pdf' | 'xml',
    file: File
  ): Promise<{ storage_path: string }> {
    const formData = new FormData();
    formData.append('file', file);

    const url = `/api/purchase-entries/${entryId}/documents?file_type=${fileType}`;

    const response = await fetch(url, {
      method: 'POST',
      body: formData,
    });

    if (response.status === 401) {
      window.location.href = '/signin';
      throw new Error('Unauthorized');
    }

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ message: response.statusText }));
      throw new Error(
        error.error || error.message || `HTTP error! status: ${response.status}`
      );
    }

    return response.json();
  },
};
