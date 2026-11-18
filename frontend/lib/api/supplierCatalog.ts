import { apiRequest } from './config';
import type {
  SupplierCatalogItem,
  AddProductToSupplierRequest,
  UpdateSupplierCatalogRequest,
  SupplierCatalogListResponse,
} from '@/types/supplierCatalog';

export const supplierCatalogApi = {
  /**
   * Get all products from a supplier's catalog
   */
  async getProductsFromSupplier(supplierId: string): Promise<SupplierCatalogItem[]> {
    const response = await apiRequest<SupplierCatalogListResponse>(
      `/suppliers/${supplierId}/products`
    );
    return response.items || [];
  },

  /**
   * Get all suppliers for a product
   */
  async getSuppliersForProduct(productId: string): Promise<SupplierCatalogItem[]> {
    const response = await apiRequest<SupplierCatalogListResponse>(
      `/products/${productId}/suppliers`
    );
    return response.items || [];
  },

  /**
   * Add a product to a supplier's catalog
   */
  async addProductToSupplier(
    supplierId: string,
    data: AddProductToSupplierRequest
  ): Promise<SupplierCatalogItem> {
    return apiRequest<SupplierCatalogItem>(`/suppliers/${supplierId}/products`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  /**
   * Update a product's pricing in supplier's catalog
   */
  async updateProductInCatalog(
    supplierId: string,
    productId: string,
    data: UpdateSupplierCatalogRequest
  ): Promise<SupplierCatalogItem> {
    return apiRequest<SupplierCatalogItem>(
      `/suppliers/${supplierId}/products/${productId}`,
      {
        method: 'PUT',
        body: JSON.stringify(data),
      }
    );
  },

  /**
   * Remove a product from supplier's catalog
   */
  async removeProductFromSupplier(
    supplierId: string,
    productId: string
  ): Promise<void> {
    return apiRequest<void>(`/suppliers/${supplierId}/products/${productId}`, {
      method: 'DELETE',
    });
  },
};
