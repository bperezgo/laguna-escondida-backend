import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import PaymentPageClient from "@/components/signup/PaymentPageClient";
import { SETUP_COOKIE } from "@/lib/auth/sessionDecision";
import { fetchPlanPrices } from "@/lib/billing/prices";

export const dynamic = "force-dynamic";

export default async function SignupPaymentPage() {
  if (!(await cookies()).get(SETUP_COOKIE)?.value) redirect("/signin");
  return <PaymentPageClient prices={await fetchPlanPrices()} />;
}
