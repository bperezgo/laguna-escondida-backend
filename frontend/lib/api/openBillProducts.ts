import { apiRequest } from "./config";

/**
 * Mark an open bill product as completed
 */
export async function completeOpenBillProduct(
  openBillProductId: string
): Promise<void> {
  return apiRequest<void>(`/open-bill-products/${openBillProductId}/complete`, {
    method: "PATCH",
  });
}

/**
 * Set an open bill product to in-progress
 */
export async function setOpenBillProductInProgress(
  openBillProductId: string
): Promise<void> {
  return apiRequest<void>(`/open-bill-products/${openBillProductId}/in-progress`, {
    method: "PATCH",
  });
}

/**
 * Cancel an open bill product
 */
export async function cancelOpenBillProduct(
  openBillProductId: string
): Promise<void> {
  return apiRequest<void>(`/open-bill-products/${openBillProductId}/cancel`, {
    method: "PATCH",
  });
}
