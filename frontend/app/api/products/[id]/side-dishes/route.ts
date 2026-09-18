import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// GET /api/products/[id]/side-dishes - list a composite product's side-dish options
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    // Pass the backend response through verbatim (status + body), so the client
    // sees the wrapped { side_dishes: [...] } payload and any error message.
    return await serverApiRequest(`/products/${id}/side-dishes`);
  } catch (error) {
    console.error("Error fetching side dishes:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch side dishes",
      },
      { status: 500 },
    );
  }
}
