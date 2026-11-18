import { NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { RolesListResponse } from "@/types/user";

// GET /api/admin/roles - List all assignable roles
export async function GET() {
  const response = await serverApiRequest<RolesListResponse>("/admin/roles");

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch roles" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}
