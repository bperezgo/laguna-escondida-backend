import { apiRequest } from './config';
import type { ElectronicInvoice, CreateElectronicInvoiceRequest } from '@/types/invoice';

export const invoicesApi = {
  /**
   * Create a new electronic invoice
   */
  async create(invoice: CreateElectronicInvoiceRequest): Promise<ElectronicInvoice> {
    return apiRequest<ElectronicInvoice>('/invoices', {
      method: 'POST',
      body: JSON.stringify(invoice),
    });
  },
};

