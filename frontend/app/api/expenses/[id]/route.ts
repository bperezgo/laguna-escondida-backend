import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { Expense, UpdateExpenseRequest } from "@/types/expense";

// GET /api/expenses/:id - Get expense by ID
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<Expense>(`/expenses/${id}`);

  if (!response.ok) {
    return NextResponse.json(
      { error: "Expense not found" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// PUT /api/expenses/:id - Update expense
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateExpenseRequest = await request.json();
    const response = await serverApiRequest<Expense>(`/expenses/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to update expense" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating expense:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to update expense",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/expenses/:id - Delete expense
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const response = await serverApiRequest(`/expenses/${id}`, {
      method: "DELETE",
    });

    if (!response.ok && response.status !== 204) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to delete expense" },
        { status: response.status }
      );
    }

    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error deleting expense:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to delete expense",
      },
      { status: 500 }
    );
  }
}
