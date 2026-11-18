import { serverApiRequest } from "@/lib/api/server";
import type { PurchaseEntry, PurchaseEntryListResponse } from "@/types/purchaseEntry";
import type { Supplier, SupplierListResponse } from "@/types/supplier";
import PurchaseEntriesPageClient from "@/components/purchase-entries/PurchaseEntriesPageClient";

export default async function PurchaseEntriesPage() {
  let entries: PurchaseEntry[] = [];
  let suppliers: Supplier[] = [];

  try {
    const [entriesRes, suppliersRes] = await Promise.all([
      serverApiRequest<PurchaseEntryListResponse>("/purchase-entries"),
      serverApiRequest<SupplierListResponse>("/suppliers"),
    ]);

    const entriesData = await entriesRes.json();
    const suppliersData = await suppliersRes.json();

    entries = entriesData.entries || [];
    suppliers = suppliersData.suppliers || [];
  } catch (error) {
    console.error("Error fetching purchase entries data:", error);
  }

  return (
    <PurchaseEntriesPageClient
      initialEntries={entries}
      initialSuppliers={suppliers}
    />
  );
}
