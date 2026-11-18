import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  DeliveryZone,
  UpdateDeliveryZoneRequest,
} from "@/types/deliveryZone";

// PUT /api/delivery-zones/:id - Update name, boundary or active flag
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateDeliveryZoneRequest = await request.json();
    const response = await serverApiRequest<DeliveryZone>(
      `/delivery-zones/${id}`,
      {
        method: "PUT",
        body: JSON.stringify(body),
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        {
          error: errorData.error || "Failed to update delivery zone",
          code: errorData.code,
        },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating delivery zone:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update delivery zone",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/delivery-zones/:id - Soft-delete a zone
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const response = await serverApiRequest<{ message: string }>(
      `/delivery-zones/${id}`,
      { method: "DELETE" }
    );

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to delete delivery zone" },
        { status: response.status }
      );
    }

    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error deleting delivery zone:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to delete delivery zone",
      },
      { status: 500 }
    );
  }
}
