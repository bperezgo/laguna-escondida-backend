import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// GET /api/open-bill-products/:area/pending
// Lines still on the area's board (the same set the SSE stream replays on connect).
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ area: string }> }
) {
  const { area } = await params;

  return await serverApiRequest<{ products: unknown[] }>(
    `/open-bill-products/${area}/pending`,
    {
      method: "GET",
    }
  );
}
