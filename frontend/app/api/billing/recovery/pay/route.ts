import { NextRequest, NextResponse } from "next/server";
import { payRequestBody, recoveryDecision } from "@/lib/billing/recoveryDecision";
import { backendSetupResponse } from "@/lib/billing/setupResponse";

// An empty body pays with the saved card. A new card arrives as Wompi's token, never the card.
export async function POST(request: NextRequest) {
  const body = payRequestBody(await request.json().catch(() => null));
  if (!body) {
    return NextResponse.json({ error: "A new card needs card_token and both acceptance tokens", field: "card_token" }, { status: 400 });
  }
  return backendSetupResponse("POST", "/billing/recovery/pay", body, recoveryDecision);
}
