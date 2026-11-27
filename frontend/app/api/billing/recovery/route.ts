import { recoveryStateDecision } from "@/lib/billing/recoveryDecision";
import { backendSetupResponse } from "@/lib/billing/setupResponse";

// Once the payment is approved this answers `{ signed_in: true }` and sets the session cookie.
export async function GET() {
  return backendSetupResponse("GET", "/billing/recovery", undefined, recoveryStateDecision);
}
