import { NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { EdgeStatus } from "@/types/edge";

// GET /api/edge/status - Edge node connectivity & sync status
export async function GET() {
  try {
    const response = await serverApiRequest<EdgeStatus>("/edge/status");
    if (!response.ok) {
      return NextResponse.json(
        { error: "Failed to fetch edge status" },
        { status: response.status }
      );
    }
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    // Backend unreachable — the client treats any non-ok response as offline.
    console.error("Error fetching edge status:", error);
    return NextResponse.json(
      { error: "Edge node unreachable" },
      { status: 502 }
    );
  }
}
