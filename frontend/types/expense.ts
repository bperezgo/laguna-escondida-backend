export interface ExpenseCategory {
  id: string;
  code: string;
  name: string;
  description?: string | null;
  is_active: boolean;
  created_at: string;
}

export interface CreateExpenseCategoryRequest {
  code: string;
  name: string;
  description?: string | null;
}

export interface UpdateExpenseCategoryRequest {
  code: string;
  name: string;
  description?: string | null;
  is_active: boolean;
}

export interface ExpenseCategoryListResponse {
  categories: ExpenseCategory[];
  total: number;
}

export interface Expense {
  id: string;
  category_id: string;
  category_code?: string;
  category_name?: string;
  supplier_id?: string | null;
  supplier_name?: string | null;
  amount: string;
  description: string;
  expense_date: string;
  reference?: string | null;
  notes?: string | null;
  pdf_storage_path?: string | null;
  xml_storage_path?: string | null;
  created_at: string;
}

export interface CreateExpenseRequest {
  category_id: string;
  supplier_id?: string | null;
  amount: string;
  description: string;
  expense_date?: string | null;
  reference?: string | null;
  notes?: string | null;
}

export interface UpdateExpenseRequest {
  category_id: string;
  supplier_id?: string | null;
  amount: string;
  description: string;
  expense_date?: string | null;
  reference?: string | null;
  notes?: string | null;
}

export interface ExpenseListResponse {
  expenses: Expense[];
  total: number;
}

export interface ExpenseFilters {
  category_id?: string;
  supplier_id?: string;
  start_date?: string;
  end_date?: string;
}

export interface ExpenseDocumentUploadResponse {
  storage_path: string;
}

export interface ExpenseFormData extends CreateExpenseRequest {
  pdfFile?: File | null;
  xmlFile?: File | null;
}
