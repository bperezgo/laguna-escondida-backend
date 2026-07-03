import { apiRequest } from "./config";
import type { OpenBillProductFromSSE } from "@/types/commandItem";

/**
 * Fetch today's fully-completed comandas for an area (read-only "Comandas Listas").
 * "Today" is the local business day (America/Bogota), computed server-side.
 */
export async function getCompletedOpenBillProducts(
  area: string,
): Promise<OpenBillProductFromSSE[]> {
  const data = await apiRequest<{ products: OpenBillProductFromSSE[] }>(
    `/open-bill-products/${area}/completed`,
  );
  return data.products ?? [];
}

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
 * Revert a completed open bill product back to created (undo a kitchen strike-through)
 */
export async function uncompleteOpenBillProduct(
  orderId: string,
  openBillProductId: string,
): Promise<void> {
  return apiRequest<void>(
    `/orders/${orderId}/products/${openBillProductId}/uncomplete`,
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
