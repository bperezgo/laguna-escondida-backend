export interface Supplier {
  id: string;
  name: string;
  identification_type?: string | null;
  identification_number?: string | null;
  contact_name?: string | null;
  phone?: string | null;
  email?: string | null;
  notes?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateSupplierRequest {
  name: string;
  identification_type?: string | null;
  identification_number?: string | null;
  contact_name?: string | null;
  phone?: string | null;
  email?: string | null;
  notes?: string | null;
}

export interface UpdateSupplierRequest {
  name: string;
  identification_type?: string | null;
  identification_number?: string | null;
  contact_name?: string | null;
  phone?: string | null;
  email?: string | null;
  notes?: string | null;
}

export interface SupplierListResponse {
  suppliers: Supplier[];
  total: number;
}
