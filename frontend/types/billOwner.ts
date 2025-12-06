export interface BillOwner {
  id: string;
  name: string;
  email?: string | null;
  phone?: string | null;
  address?: string | null;
  identification?: string | null;
  identification_type?: string | null;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface BillOwnerResponse {
  bill_owner: BillOwner;
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
  bill_owner?: {
    name: string;
    email?: string | null;
    phone?: string | null;
    address?: string | null;
    identification?: string | null;
    identification_type?: string | null;
  } | null;
}
