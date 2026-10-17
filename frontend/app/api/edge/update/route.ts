import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import type { EdgeUpdateStatus } from "@/types/edge";

// Calls the backend directly rather than via serverApiRequest: the 404 for a
// missing route and the 202 for a request may have non-JSON bodies.
async function backendFetch(init?: RequestInit): Promise<Response> {
  const token = await getAccessToken();
  const response = await fetch(`${config.apiUrl}/edge/update`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    cache: "no-store",
  });
  if (response.status === 401) {
    const cookieStore = await cookies();
    cookieStore.delete("access_token");
  }
  return response;
}

async function readJson(response: Response): Promise<unknown> {
  return response.json().catch(() => ({}));
}

// GET /api/edge/update - Staged release and update progress
export async function GET() {
  try {
    const response = await backendFetch();
    // Feature off, older edge-node or no permission: report "disabled" quietly.
    if (response.status === 403 || response.status === 404) {
      return NextResponse.json<EdgeUpdateStatus>({ enabled: false });
    }
    return NextResponse.json(await readJson(response), {
      status: response.status,
    });
  } catch (error) {
    // Expected while the box applies an update and edge-node is restarting.
    console.error("Error fetching edge update status:", error);
    return NextResponse.json(
      { error: "Edge node unreachable" },
      { status: 503 }
    );
  }
}

// POST /api/edge/update - Ask the updater to apply the staged release
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const response = await backendFetch({
      method: "POST",
      body: JSON.stringify(body),
    });
    return NextResponse.json(await readJson(response), {
      status: response.status,
    });
  } catch (error) {
    console.error("Error requesting edge update:", error);
    return NextResponse.json(
      { error: "Edge node unreachable" },
      { status: 503 }
    );
  }
}
