import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { IssuedOrganizationApiKey, OrganizationApiKeyListResponse } from "@/types/organizationApiKeys";
import { integrationErrorResponse } from "../integrations/errors";

// GET /api/organization/api-keys - The organization's keys, never their plaintext
export async function GET() {
  const response = await serverApiRequest<OrganizationApiKeyListResponse>("/organization/api-keys");
  if (!response.ok) {
    return integrationErrorResponse(response, "Failed to fetch API keys");
  }
  return NextResponse.json(await response.json());
}

// POST /api/organization/api-keys - Issue a key; the response carries its plaintext once
export async function POST(request: NextRequest) {
  const body = await request.text();
  try {
    const response = await serverApiRequest<IssuedOrganizationApiKey>("/organization/api-keys", {
      method: "POST",
      body,
    });
    if (!response.ok) {
      return integrationErrorResponse(response, "Failed to issue API key");
    }
    return NextResponse.json(await response.json(), { status: 201 });
  } catch {
    return NextResponse.json({ error: "Failed to issue API key" }, { status: 502 });
  }
}
