import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationIntegration } from "@/types/organizationIntegrations";
import { integrationErrorResponse } from "../errors";

type Params = { params: Promise<{ provider: string }> };

function backendPath(provider: string) {
  return `/organization/integrations/${encodeURIComponent(provider)}`;
}

// GET /api/organization/integrations/:provider
export async function GET(_request: NextRequest, { params }: Params) {
  const { provider } = await params;
  const response = await serverApiRequest<OrganizationIntegration>(backendPath(provider));
  if (!response.ok) {
    return integrationErrorResponse(response, "Failed to fetch integration");
  }
  return NextResponse.json(await response.json());
}

// PUT /api/organization/integrations/:provider - Create or replace the keys
export async function PUT(request: NextRequest, { params }: Params) {
  const { provider } = await params;
  const body = await request.text();
  try {
    const response = await serverApiRequest<OrganizationIntegration>(backendPath(provider), {
      method: "PUT",
      body,
    });
    if (!response.ok) {
      return integrationErrorResponse(response, "Failed to save integration");
    }
    return NextResponse.json(await response.json());
  } catch {
    return NextResponse.json({ error: "Failed to save integration" }, { status: 502 });
  }
}

// DELETE /api/organization/integrations/:provider
export async function DELETE(_request: NextRequest, { params }: Params) {
  const { provider } = await params;
  const response = await serverApiRequest(backendPath(provider), { method: "DELETE" });
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 });
  }
  return integrationErrorResponse(response, "Failed to remove integration");
}
