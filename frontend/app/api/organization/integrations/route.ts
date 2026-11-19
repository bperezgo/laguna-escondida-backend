import { NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationIntegrationListResponse } from "@/types/organizationIntegrations";
import { integrationErrorResponse } from "./errors";

// GET /api/organization/integrations - The organization's integrations, with hints only
export async function GET() {
  const response = await serverApiRequest<OrganizationIntegrationListResponse>("/organization/integrations");
  if (!response.ok) {
    return integrationErrorResponse(response, "Failed to fetch integrations");
  }
  return NextResponse.json(await response.json());
}
