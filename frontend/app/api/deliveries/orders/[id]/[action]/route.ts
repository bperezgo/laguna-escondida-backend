import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryAction, DeliveryOrder } from "@/types/delivery";

const ACTIONS: readonly DeliveryAction[] = ["dispatch", "deliver", "fail"];

function isAction(value: string): value is DeliveryAction {
  return (ACTIONS as readonly string[]).includes(value);
}

// POST /api/deliveries/orders/:id/{dispatch|deliver|fail} - Move an order on its way
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; action: string }> }
) {
  const { id, action } = await params;
  if (!isAction(action)) {
    return NextResponse.json({ error: "Unknown action" }, { status: 404 });
  }
  const body = await request.text();
  return serverApiRequest<DeliveryOrder>(
    `/deliveries/orders/${encodeURIComponent(id)}/${action}`,
    { method: "POST", body: body || undefined }
  );
}
