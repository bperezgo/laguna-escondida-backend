import SignupPageClient from "@/components/signup/SignupPageClient";
import { fetchPlanPrices } from "@/lib/billing/prices";
import { planFromQuery } from "@/lib/plans";

export const dynamic = "force-dynamic";

export default async function SignupPage({ searchParams }: { searchParams: Promise<{ plan?: string | string[] }> }) {
  const [{ plan }, prices] = await Promise.all([searchParams, fetchPlanPrices()]);
  return <SignupPageClient initialPlan={planFromQuery(plan)} prices={prices} />;
}
