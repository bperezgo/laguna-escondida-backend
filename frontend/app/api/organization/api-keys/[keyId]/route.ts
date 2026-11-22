import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import { integrationErrorResponse } from "../../integrations/errors";

type Params = { params: Promise<{ keyId: string }> };

// DELETE /api/organization/api-keys/:keyId - Revoke a key (permanent)
export async function DELETE(_request: NextRequest, { params }: Params) {
  const { keyId } = await params;
  const response = await serverApiRequest(`/organization/api-keys/${encodeURIComponent(keyId)}`, { method: "DELETE" });
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 });
  }
  return integrationErrorResponse(response, "Failed to revoke API key");
}
