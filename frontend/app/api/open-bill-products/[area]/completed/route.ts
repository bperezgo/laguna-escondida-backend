import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// GET /api/open-bill-products/:area/completed
// Today's fully-completed comandas for the area (local business day, computed by the backend).
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ area: string }> }
) {
  const { area } = await params;

  return await serverApiRequest<{ products: unknown[] }>(
    `/open-bill-products/${area}/completed`,
    {
      method: "GET",
    }
  );
}
