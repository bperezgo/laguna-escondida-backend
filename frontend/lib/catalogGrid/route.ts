import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { forwardCatalogImport } from "@/lib/catalogGrid/proxy";

export function catalogImportHandler(endpoint: string) {
  return async function POST(request: NextRequest) {
    try {
      const forwarded = await forwardCatalogImport(
        fetch,
        `${config.apiUrl}${endpoint}`,
        await getAccessToken(),
        await request.text(),
      );
      if (forwarded.status === 401) (await cookies()).delete("access_token");
      return new NextResponse(forwarded.body, {
        status: forwarded.status,
        headers: { "Content-Type": forwarded.contentType, "Cache-Control": "no-store" },
      });
    } catch (error) {
      console.error(`Request to ${endpoint} failed:`, error);
      return NextResponse.json({ error: "Backend unavailable" }, { status: 502 });
    }
  };
}
