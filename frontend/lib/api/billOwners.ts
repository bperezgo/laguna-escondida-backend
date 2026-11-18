import { apiRequest } from "./config";
import type { BillOwnerResponse } from "@/types/billOwner";

export async function getBillOwnerById(id: string): Promise<BillOwnerResponse> {
  return apiRequest<BillOwnerResponse>(`/bill-owners/${id}`);
}
