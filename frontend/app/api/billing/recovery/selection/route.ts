import { NextRequest } from "next/server";
import { recoveryDecision } from "@/lib/billing/recoveryDecision";
import { backendSetupResponse } from "@/lib/billing/setupResponse";

export async function PUT(request: NextRequest) {
  const { plan, period } = await request.json().catch(() => ({}));
  return backendSetupResponse("PUT", "/billing/recovery/selection", { plan, period }, recoveryDecision);
}
