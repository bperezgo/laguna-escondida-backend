import { apiRequest } from "./config";

/**
 * Mark an open bill product as completed
 */
export async function completeOpenBillProduct(
  orderId: string,
  openBillProductId: string,
): Promise<void> {
  return apiRequest<void>(
    `/orders/${orderId}/products/${openBillProductId}/complete`,
    {
      method: "PATCH",
    },
  );
}

/**
 * Set an open bill product to in-progress
 */
export async function setOpenBillProductInProgress(
  orderId: string,
  openBillProductId: string,
): Promise<void> {
  return apiRequest<void>(
    `/orders/${orderId}/products/${openBillProductId}/in-progress`,
    {
      method: "PATCH",
    },
  );
}

/**
 * Cancel an open bill product
 */
export async function cancelOpenBillProduct(
  orderId: string,
  openBillProductId: string,
): Promise<void> {
  return apiRequest<void>(
    `/orders/${orderId}/products/${openBillProductId}/cancel`,
    {
      method: "PATCH",
    },
  );
}
