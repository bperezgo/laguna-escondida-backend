export type ElectronicInvoicePaymentCode =
  | 'credit_card'
  | 'debit_card'
  | 'cash'
  | 'transfer_debit_bank'
  | 'transfer_credit_bank'
  | 'transfer_debit_interbank';

export type DocumentType = 'CC' | 'NIT';

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
  quantity: string;
  unitPrice: string;
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

