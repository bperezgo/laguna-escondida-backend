import { apiRequest } from './config';
import type {
  PurchaseEntry,
  CreatePurchaseEntryRequest,
  PurchaseEntryListResponse,
  PurchaseEntryFilters,
} from '@/types/purchaseEntry';

export const purchaseEntriesApi = {
  /**
   * Get all purchase entries with optional filters
   */
  async getAll(filters?: PurchaseEntryFilters): Promise<PurchaseEntry[]> {
    const params = new URLSearchParams();
    if (filters?.supplier_id) params.append('supplier_id', filters.supplier_id);
    if (filters?.start_date) params.append('start_date', filters.start_date);
    if (filters?.end_date) params.append('end_date', filters.end_date);
    const queryString = params.toString();
    const endpoint = `/purchase-entries${queryString ? `?${queryString}` : ''}`;
    const response = await apiRequest<PurchaseEntryListResponse>(endpoint);
    return response.entries || [];
  },

  /**
   * Export purchase entries as CSV file
   */
  async exportCSV(filters?: PurchaseEntryFilters): Promise<void> {
    const params = new URLSearchParams();
    if (filters?.supplier_id) params.append('supplier_id', filters.supplier_id);
    if (filters?.start_date) params.append('start_date', filters.start_date);
    if (filters?.end_date) params.append('end_date', filters.end_date);
    const queryString = params.toString();
    const endpoint = `/api/purchase-entries/export${queryString ? `?${queryString}` : ''}`;

    const response = await fetch(endpoint, { method: 'GET' });

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

    const contentDisposition = response.headers.get('Content-Disposition');
    let filename = `entradas_compra_${new Date().toISOString().split('T')[0]}.csv`;
    if (contentDisposition) {
      const match = contentDisposition.match(/filename=([^;]+)/);
      if (match) filename = match[1].replace(/"/g, '');
    }

    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
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

  /**
   * Upload a ZIP file containing both PDF and XML for a purchase entry
   */
  async uploadZipDocument(
    entryId: string,
    file: File
  ): Promise<{ pdf_storage_path: string; xml_storage_path: string }> {
    const formData = new FormData();
    formData.append('file', file);

    const url = `/api/purchase-entries/${entryId}/documents`;

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
