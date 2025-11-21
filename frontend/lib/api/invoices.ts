import { apiRequest } from "./config";
import type {
  ElectronicInvoice,
  CreateElectronicInvoiceRequest,
  InvoiceFilters,
  InvoiceListResponse,
} from "@/types/invoice";

export const invoicesApi = {
  /**
   * Get all invoices with optional filters
   */
  async getAll(filters?: InvoiceFilters): Promise<InvoiceListResponse> {
    const params = new URLSearchParams();

    if (filters?.page) {
      params.set("page", filters.page.toString());
    }
    if (filters?.page_size) {
      params.set("page_size", filters.page_size.toString());
    }
    if (filters?.created_at_start) {
      params.set("created_at_start", filters.created_at_start);
    }
    if (filters?.created_at_end) {
      params.set("created_at_end", filters.created_at_end);
    }
    if (filters?.national_identification) {
      params.set("national_identification", filters.national_identification);
    }

    const queryString = params.toString();
    const endpoint = queryString ? `/invoices?${queryString}` : "/invoices";

    return apiRequest<InvoiceListResponse>(endpoint, {
      method: "GET",
    });
  },

  /**
   * Create a new electronic invoice
   */
  async create(
    invoice: CreateElectronicInvoiceRequest
  ): Promise<ElectronicInvoice> {
    return apiRequest<ElectronicInvoice>("/invoices", {
      method: "POST",
      body: JSON.stringify(invoice),
    });
  },
};
