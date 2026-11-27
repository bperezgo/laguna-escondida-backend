import DonePageClient from "@/components/signup/DonePageClient";
import { fetchPlanPrices } from "@/lib/billing/prices";

export const dynamic = "force-dynamic";

export default async function SignupDonePage() {
  return <DonePageClient prices={await fetchPlanPrices()} />;
}
