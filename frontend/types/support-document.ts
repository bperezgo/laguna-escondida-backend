export type SupportDocumentPaymentCode =
  | "credit_card"
  | "debit_card"
  | "cash"
  | "transfer_debit_bank"
  | "transfer_credit_bank"
  | "transfer_debit_interbank";

export type ProviderDocumentType = "CC" | "NIT";

export interface Provider {
  id: string;
  document_type: ProviderDocumentType;
  name: string;
  email: string;
}

export interface SupportDocumentItem {
  quantity: number;
  description: string;
  price: number;
}

export interface CreateSupportDocumentRequest {
  payment_code: SupportDocumentPaymentCode;
  provider: Provider;
  items: SupportDocumentItem[];
}

export interface SupportDocumentFilters {
  page?: number;
  page_size?: number;
  created_at_start?: string;
  created_at_end?: string;
  provider_document_number?: string;
}

export interface SupportDocumentListItem {
  id: string;
  total_amount: number;
  discount_amount: number;
  vat: number;
  ico: number;
  tip: number;
  cufe: string;
  tascode: string;
  provider_document_number: string;
  provider_name: string;
  pdf_download_url: string | null;
  xml_download_url: string | null;
  created_at: string;
}

export interface SupportDocumentListResponse {
  support_documents: SupportDocumentListItem[];
  total_count: number;
  page: number;
  page_size: number;
  total_pages: number;
}
