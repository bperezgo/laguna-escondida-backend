export type ElectronicInvoicePaymentCode =
  | "credit_card"
  | "debit_card"
  | "cash"
  | "transfer_debit_bank"
  | "transfer_credit_bank"
  | "transfer_debit_interbank";

export type DocumentType = "CC" | "NIT";

export interface Customer {
  id: string; // documentNumber
  document_type: DocumentType;
  name: string;
  email: string;
}

export interface InvoiceAmounts {
  totalAmount: string;
  discountAmount: string;
  taxAmount: string;
  payAmount: string;
}

export interface InvoiceAllowance {
  charge: string;
  reasonCode: string;
  description: string;
  baseAmount: string;
  amount: string;
}

export interface InvoiceTax {
  ID: string;
  taxAmount: string;
  percent: string;
}

export interface InvoiceItem {
  product_id: string;
  quantity: number;
  totalPriceWithTaxes: string;
  total: string;
  description: string;
  brand: string;
  model: string;
  code: string;
  allowance?: InvoiceAllowance[];
  taxes?: InvoiceTax[];
}

export interface ElectronicInvoice {
  consecutive: number;
  issue_date: string;
  issue_time: string;
  payment_code: ElectronicInvoicePaymentCode;
  customer?: Customer;
  amounts: InvoiceAmounts;
  items: InvoiceItem[];
}

export interface CreateElectronicInvoiceRequest {
  payment_code: ElectronicInvoicePaymentCode;
  customer?: Customer;
  items: InvoiceItem[];
}

export interface InvoiceFilters {
  page?: number;
  page_size?: number;
  created_at_start?: string; // RFC3339 format
  created_at_end?: string; // RFC3339 format
  national_identification?: string;
}

export interface InvoiceListItem {
  id: string;
  total_amount: number;
  discount_amount: number;
  vat: number;
  ico: number;
  tip: number;
  document_url: string | null;
  cufe: string;
  tascode: string;
  customer_id: string | null;
  created_at: string;
}

export interface InvoiceListResponse {
  invoices: InvoiceListItem[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}
