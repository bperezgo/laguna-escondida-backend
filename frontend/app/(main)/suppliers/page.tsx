import { serverApiRequest } from "@/lib/api/server";
import type { Supplier, SupplierListResponse } from "@/types/supplier";
import SuppliersPageClient from "@/components/suppliers/SuppliersPageClient";

export default async function SuppliersPage() {
  let suppliers: Supplier[] = [];

  try {
    const response = await serverApiRequest<SupplierListResponse>("/suppliers");
    const data = await response.json();
    suppliers = data.suppliers || [];
  } catch (error) {
    console.error("Error fetching suppliers:", error);
  }

  return <SuppliersPageClient initialSuppliers={suppliers} />;
}
