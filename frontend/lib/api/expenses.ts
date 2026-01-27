import { apiRequest } from './config';
import type {
  Expense,
  ExpenseCategory,
  CreateExpenseRequest,
  UpdateExpenseRequest,
  CreateExpenseCategoryRequest,
  UpdateExpenseCategoryRequest,
  ExpenseListResponse,
  ExpenseCategoryListResponse,
  ExpenseFilters,
} from '@/types/expense';

export const expenseCategoriesApi = {
  /**
   * Get all expense categories
   */
  async getAll(): Promise<ExpenseCategory[]> {
    const response = await apiRequest<ExpenseCategoryListResponse>('/expense-categories');
    return response.categories || [];
  },

  /**
   * Get an expense category by ID
   */
  async getById(id: string): Promise<ExpenseCategory> {
    return apiRequest<ExpenseCategory>(`/expense-categories/${id}`);
  },

  /**
   * Create a new expense category
   */
  async create(data: CreateExpenseCategoryRequest): Promise<ExpenseCategory> {
    return apiRequest<ExpenseCategory>('/expense-categories', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * Update an expense category
   */
  async update(id: string, data: UpdateExpenseCategoryRequest): Promise<ExpenseCategory> {
    return apiRequest<ExpenseCategory>(`/expense-categories/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  },
};

export const expensesApi = {
  /**
   * Get all expenses with optional filters
   */
  async getAll(filters?: ExpenseFilters): Promise<Expense[]> {
    const params = new URLSearchParams();
    if (filters?.category_id) params.append('category_id', filters.category_id);
    if (filters?.supplier_id) params.append('supplier_id', filters.supplier_id);
    if (filters?.start_date) params.append('start_date', filters.start_date);
    if (filters?.end_date) params.append('end_date', filters.end_date);

    const queryString = params.toString();
    const endpoint = `/expenses${queryString ? `?${queryString}` : ''}`;
    
    const response = await apiRequest<ExpenseListResponse>(endpoint);
    return response.expenses || [];
  },

  /**
   * Get an expense by ID
   */
  async getById(id: string): Promise<Expense> {
    return apiRequest<Expense>(`/expenses/${id}`);
  },

  /**
   * Create a new expense
   */
  async create(data: CreateExpenseRequest): Promise<Expense> {
    return apiRequest<Expense>('/expenses', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * Update an expense
   */
  async update(id: string, data: UpdateExpenseRequest): Promise<Expense> {
    return apiRequest<Expense>(`/expenses/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  },

  /**
   * Delete an expense
   */
  async delete(id: string): Promise<void> {
    await apiRequest(`/expenses/${id}`, {
      method: 'DELETE',
    });
  },

  /**
   * Upload a document (PDF or XML) for an expense
   */
  async uploadDocument(
    expenseId: string,
    categoryCode: string,
    fileType: 'pdf' | 'xml',
    file: File
  ): Promise<{ storage_path: string }> {
    const formData = new FormData();
    formData.append('file', file);

    const url = `/api/expenses/${expenseId}/documents?category_code=${encodeURIComponent(categoryCode)}&file_type=${fileType}`;

    const response = await fetch(url, {
      method: 'POST',
      body: formData,
    });

    if (response.status === 401) {
      window.location.href = '/signin';
      throw new Error('Unauthorized');
    }

    if (!response.ok) {
      const error = await response
        .json()
        .catch(() => ({ message: response.statusText }));
      throw new Error(
        error.error || error.message || `HTTP error! status: ${response.status}`
      );
    }

    return response.json();
  },
};
