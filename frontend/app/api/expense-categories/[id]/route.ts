import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  ExpenseCategory,
  UpdateExpenseCategoryRequest,
} from "@/types/expense";

// GET /api/expense-categories/:id - Get expense category by ID
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<ExpenseCategory>(
    `/expense-categories/${id}`
  );

  if (!response.ok) {
    return NextResponse.json(
      { error: "Expense category not found" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// PUT /api/expense-categories/:id - Update expense category
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateExpenseCategoryRequest = await request.json();
    const response = await serverApiRequest<ExpenseCategory>(
      `/expense-categories/${id}`,
      {
        method: "PUT",
        body: JSON.stringify(body),
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to update expense category" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating expense category:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update expense category",
      },
      { status: 500 }
    );
  }
}
