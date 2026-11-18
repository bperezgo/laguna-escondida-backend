import { apiRequest } from "./config";
import type {
  CreateSupportDocumentRequest,
  SupportDocumentFilters,
  SupportDocumentListResponse,
} from "@/types/support-document";

export const supportDocumentsApi = {
  async getAll(
    filters?: SupportDocumentFilters
  ): Promise<SupportDocumentListResponse> {
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
    if (filters?.provider_document_number) {
      params.set("provider_document_number", filters.provider_document_number);
    }

    const queryString = params.toString();
    const endpoint = queryString
      ? `/support-documents?${queryString}`
      : "/support-documents";

    return apiRequest<SupportDocumentListResponse>(endpoint, {
      method: "GET",
    });
  },

  async create(
    data: CreateSupportDocumentRequest
  ): Promise<{ message: string }> {
    return apiRequest<{ message: string }>("/support-documents", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async exportCSV(filters?: SupportDocumentFilters): Promise<void> {
    const params = new URLSearchParams();

    if (filters?.created_at_start) {
      params.set("created_at_start", filters.created_at_start);
    }
    if (filters?.created_at_end) {
      params.set("created_at_end", filters.created_at_end);
    }
    if (filters?.provider_document_number) {
      params.set("provider_document_number", filters.provider_document_number);
    }

    const queryString = params.toString();
    const endpoint = queryString
      ? `/api/support-documents/export?${queryString}`
      : "/api/support-documents/export";

    const response = await fetch(endpoint, {
      method: "GET",
    });

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

    const contentDisposition = response.headers.get("Content-Disposition");
    let filename = `documentos_soporte_${new Date().toISOString().split("T")[0]}.csv`;

    if (contentDisposition) {
      const match = contentDisposition.match(/filename=([^;]+)/);
      if (match) {
        filename = match[1].replace(/"/g, "");
      }
    }

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
