import { NextRequest } from "next/server";
import { backendSetupResponse } from "@/lib/billing/setupResponse";

export async function PUT(request: NextRequest) {
  const { plan, period } = await request.json().catch(() => ({}));
  return backendSetupResponse("PUT", "/billing/setup/selection", { plan, period });
}
