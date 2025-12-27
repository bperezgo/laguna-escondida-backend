import { apiRequest } from "./config";

/**
 * Mark a command item as completed
 */
export async function completeCommandItem(
  openBillProductId: string
): Promise<void> {
  return apiRequest<void>(`/command-items/${openBillProductId}/complete`, {
    method: "PATCH",
  });
}

