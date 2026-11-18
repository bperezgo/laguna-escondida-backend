export interface BillOwner {
  id: string; // This is the identification number
  name: string;
  email?: string | null;
  celphone?: string | null; // API uses 'celphone' not 'phone'
  address?: string | null;
  identification_type?: string | null;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface BillOwnerResponse {
  bill_owner?: BillOwner; // Make it optional to handle direct responses
}

export type PaymentType =
  | "credit_card"
  | "debit_card"
  | "cash"
  | "transfer_debit_bank"
  | "transfer_credit_bank"
  | "transfer_debit_interbank";

export interface PayOrderRequest {
  order_id: string;
  payment_type: PaymentType;
  customer?: {
    name: string;
    email: string;
    document_type: string;
    id: string;
  } | null;
}
