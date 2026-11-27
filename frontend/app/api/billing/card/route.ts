import { NextRequest, NextResponse } from "next/server";
import { serverApiPassthrough } from "@/lib/api/server";
import { cardRequestBody } from "@/lib/billing/setupDecision";

// Receives Wompi's token for the card, never the card: anything else in the body is dropped.
export async function PUT(request: NextRequest) {
  const body = cardRequestBody(await request.json().catch(() => null));
  if (!body) {
    return NextResponse.json({ error: "card_token and both acceptance tokens are required" }, { status: 400 });
  }
  return serverApiPassthrough("/billing/card", { method: "PUT", body: JSON.stringify(body) });
}
