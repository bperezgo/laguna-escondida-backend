import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { PaymentSettings } from "@/types/paymentSettings";

// Passes the backend's error, code and field through, so the page can point at the rejected field.
async function errorResponse(response: Response, fallback: string) {
  const errorData = await response.json().catch(() => ({}));
  return NextResponse.json(
    { error: errorData.error || fallback, code: errorData.code, field: errorData.field },
    { status: response.status }
  );
}

// GET /api/organization/payment-settings - How agent orders are paid
export async function GET() {
  const response = await serverApiRequest<PaymentSettings>("/organization/payment-settings");
  if (!response.ok) {
    return errorResponse(response, "Failed to fetch payment settings");
  }
  return NextResponse.json(await response.json());
}

// PUT /api/organization/payment-settings - Replace the strategy, instructions and window
export async function PUT(request: NextRequest) {
  try {
    const response = await serverApiRequest<PaymentSettings>("/organization/payment-settings", {
      method: "PUT",
      body: await request.text(),
    });
    if (!response.ok) {
      return errorResponse(response, "Failed to save payment settings");
    }
    return NextResponse.json(await response.json());
  } catch {
    return NextResponse.json({ error: "Failed to save payment settings" }, { status: 502 });
  }
}
