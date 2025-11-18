import { apiRequest } from "./config";
import type {
  Stock,
  CreateStockRequest,
  AddOrDecreaseStockRequest,
  BulkStockCreationOrUpdatingRequest,
  StockListResponse,
} from "@/types/stock";

export const stockApi = {
  /**
   * Get all stocks (non-deleted)
   */
  async getAll(): Promise<Stock[]> {
    const response = await apiRequest<StockListResponse>("/stock");
    return response.stocks || [];
  },

  /**
   * Create a new stock
   */
  async create(stock: CreateStockRequest): Promise<Stock> {
    return apiRequest<Stock>("/stock", {
      method: "POST",
      body: JSON.stringify(stock),
    });
  },

  /**
   * Add or decrease stock amount
   */
  async addOrDecrease(stock: AddOrDecreaseStockRequest): Promise<Stock> {
    return apiRequest<Stock>("/stock/add-or-decrease", {
      method: "POST",
      body: JSON.stringify(stock),
    });
  },

  /**
   * Delete a stock (soft delete)
   */
  async delete(productId: string): Promise<void> {
    return apiRequest<void>(`/stock/${productId}`, {
      method: "DELETE",
    });
  },

  /**
   * Bulk create or update stocks
   */
  async bulkCreateOrUpdate(
    request: BulkStockCreationOrUpdatingRequest
  ): Promise<Stock[]> {
    const response = await apiRequest<StockListResponse>("/stock/bulk", {
      method: "POST",
      body: JSON.stringify(request),
    });
    return response.stocks || [];
  },
};
