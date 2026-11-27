import { backendSetupResponse } from "@/lib/billing/setupResponse";

export async function GET() {
  return backendSetupResponse("GET", "/billing/setup");
}
