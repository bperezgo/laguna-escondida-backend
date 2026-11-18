import { apiRequest } from "./config";
import type { PrintTicketRequest, PrintTicketResponse } from "@/types/order";

/**
 * Render and print the ticket for an open bill on the edge node's physical
 * receipt printer. Only available when the backend runs in edge mode; in cloud
 * mode the endpoint returns 404 and callers should fall back to browser printing.
 */
export async function printTicket(
  data: PrintTicketRequest,
): Promise<PrintTicketResponse> {
  return apiRequest<PrintTicketResponse>("/device/print", {
    method: "POST",
    body: JSON.stringify(data),
  });
}
