import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  OrganizationSettingsResponse,
  UpdateOrganizationSettingsRequest,
} from "@/types/organizationSettings";

// Passes the backend's error, code, field and required permission through, so the page can
// point at the rejected window or say which permission is missing.
async function errorResponse(response: Response, fallback: string) {
  const errorData = await response.json().catch(() => ({}));
  return NextResponse.json(
    {
      error: errorData.error || fallback,
      code: errorData.code,
      field: errorData.field,
      required: errorData.required,
    },
    { status: response.status }
  );
}

// GET /api/organization/settings - Ordering settings plus availability right now
export async function GET() {
  const response = await serverApiRequest<OrganizationSettingsResponse>(
    "/organization/settings"
  );
  if (!response.ok) {
    return errorResponse(response, "Failed to fetch organization settings");
  }
  return NextResponse.json(await response.json());
}

// PATCH /api/organization/settings - Change the pause and/or the schedule of either type
export async function PATCH(request: NextRequest) {
  try {
    const body: UpdateOrganizationSettingsRequest = await request.json();
    const response = await serverApiRequest<OrganizationSettingsResponse>(
      "/organization/settings",
      { method: "PATCH", body: JSON.stringify(body) }
    );
    if (!response.ok) {
      return errorResponse(response, "Failed to update organization settings");
    }
    return NextResponse.json(await response.json());
  } catch (error) {
    console.error("Error updating organization settings:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update organization settings",
      },
      { status: 500 }
    );
  }
}
