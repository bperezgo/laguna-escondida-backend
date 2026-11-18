import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { PrintTicketRequest } from "@/types/order";

// POST /api/device/print - Render and print the ticket for an open bill on the
// edge node's physical receipt printer. Proxies to the Golang backend, which
// only registers this route in edge mode (404 in cloud mode).
export async function POST(request: NextRequest) {
  try {
    const body: PrintTicketRequest = await request.json();

    if (!body.open_bill_id) {
      return NextResponse.json(
        { error: "open_bill_id is required" },
        { status: 400 }
      );
    }

    return await serverApiRequest(`/device/print`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error printing ticket:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to print ticket",
      },
      { status: 500 }
    );
  }
}
