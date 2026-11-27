import { NextRequest, NextResponse } from "next/server";
import { cardDecision, cardRequestBody } from "@/lib/billing/setupDecision";
import { backendSetupResponse } from "@/lib/billing/setupResponse";

// Receives Wompi's token for the card, never the card: anything else in the body is dropped.
export async function POST(request: NextRequest) {
  const body = cardRequestBody(await request.json().catch(() => null));
  if (!body) {
    return NextResponse.json({ error: "card_token and both acceptance tokens are required" }, { status: 400 });
  }
  return backendSetupResponse("POST", "/billing/setup/card", body, cardDecision);
}
