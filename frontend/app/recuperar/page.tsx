import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import RecoveryPageClient from "@/components/billing/RecoveryPageClient";
import { SETUP_COOKIE } from "@/lib/auth/sessionDecision";
import { fetchPlanPrices } from "@/lib/billing/prices";

export const dynamic = "force-dynamic";

export default async function RecoveryPage() {
  if (!(await cookies()).get(SETUP_COOKIE)?.value) redirect("/signin");
  return <RecoveryPageClient prices={await fetchPlanPrices()} />;
}
