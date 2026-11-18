import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { cookies } from "next/headers";
import { AuthUser } from "@/lib/permissions/types";
import type { EdgeMode, EdgeStatus } from "@/types/edge";

export async function GET() {
  try {
    // Get the JWT token from cookies
    const token = await getAccessToken();

    if (!token) {
      return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
    }

    // Call the backend /auth/me endpoint
    const response = await fetch(`${config.apiUrl}/auth/me`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
    });

    // Handle unauthorized responses by clearing the invalid token
    if (response.status === 401) {
      const cookieStore = await cookies();
      cookieStore.delete("access_token");
      return NextResponse.json({ error: "Session expired" }, { status: 401 });
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({
        message: "Failed to fetch user info",
      }));
      return NextResponse.json(
        { error: error.message || "Failed to fetch user info" },
        { status: response.status },
      );
    }

    const data: AuthUser = await response.json();

    // Fetch deployment mode from the edge status endpoint.
    // Default to "cloud" on failure — more restrictive is the safer fallback.
    let deployment_mode: EdgeMode = "cloud";
    try {
      const edgeResponse = await fetch(`${config.apiUrl}/edge/status`, {
        method: "GET",
        headers: { "Content-Type": "application/json" },
      });
      if (edgeResponse.ok) {
        const edgeData: EdgeStatus = await edgeResponse.json();
        deployment_mode = edgeData.mode ?? "cloud";
      }
    } catch {
      // Backend unreachable — keep the safe "cloud" default.
    }

    return NextResponse.json({ ...data, deployment_mode });
  } catch (error) {
    console.error("Error fetching user info:", error);
    return NextResponse.json(
      { error: "Internal server error" },
      { status: 500 },
    );
  }
}
