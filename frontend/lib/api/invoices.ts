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

  /**
   * Export invoices as CSV file
   */
  async exportCSV(filters?: InvoiceFilters): Promise<void> {
    const params = new URLSearchParams();

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
    const endpoint = queryString
      ? `/api/invoices/export?${queryString}`
      : "/api/invoices/export";

    const response = await fetch(endpoint, {
      method: "GET",
    });

    // Handle unauthorized responses by redirecting to signin
    if (response.status === 401) {
      window.location.href = "/signin";
      throw new Error("Unauthorized");
    }

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ message: response.statusText }));
      throw new Error(
        error.error || error.message || `HTTP error! status: ${response.status}`
      );
    }

    // Get the filename from Content-Disposition header or generate one
    const contentDisposition = response.headers.get("Content-Disposition");
    let filename = `facturas_${new Date().toISOString().split("T")[0]}.csv`;

    if (contentDisposition) {
      const match = contentDisposition.match(/filename=([^;]+)/);
      if (match) {
        filename = match[1].replace(/"/g, "");
      }
    }

    // Get the blob and trigger download
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  },
};
