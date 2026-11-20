import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { GeneratedMarketingAppSecret } from "@/types/organizationIntegrations";
import { integrationErrorResponse } from "../../errors";

type Params = { params: Promise<{ provider: string }> };

// POST /api/organization/integrations/:provider/secret - Generate the secret. The response carries
// it, so it is passed through untouched, never logged and never cached.
export async function POST(_request: NextRequest, { params }: Params) {
  const { provider } = await params;
  try {
    const response = await serverApiRequest<GeneratedMarketingAppSecret>(
      `/organization/integrations/${encodeURIComponent(provider)}/secret`,
      { method: "POST" }
    );
    if (!response.ok) {
      return integrationErrorResponse(response, "Failed to generate secret");
    }
    return NextResponse.json(await response.json(), { headers: { "Cache-Control": "no-store" } });
  } catch {
    return NextResponse.json({ error: "Failed to generate secret" }, { status: 502 });
  }
}
