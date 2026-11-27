import { config } from "@/lib/config/config";
import { parsePlanPrices, type PlanPrices } from "@/lib/plans";

// Server side. Null on any failure, an edge box included: the page then shows no prices.
export async function fetchPlanPrices(): Promise<PlanPrices | null> {
  try {
    const response = await fetch(`${config.apiUrl}/plans`, { cache: "no-store" });
    return response.ok ? parsePlanPrices(await response.json()) : null;
  } catch {
    return null;
  }
}
