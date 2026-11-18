import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  ExpenseCategory,
  CreateExpenseCategoryRequest,
  ExpenseCategoryListResponse,
} from "@/types/expense";

// GET /api/expense-categories - Get all expense categories
export async function GET() {
  const response = await serverApiRequest<ExpenseCategoryListResponse>(
    "/expense-categories"
  );

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch expense categories" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/expense-categories - Create a new expense category
export async function POST(request: NextRequest) {
  try {
    const body: CreateExpenseCategoryRequest = await request.json();
    const response = await serverApiRequest<ExpenseCategory>(
      "/expense-categories",
      {
        method: "POST",
        body: JSON.stringify(body),
      }
    );

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to create expense category" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating expense category:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create expense category",
      },
      { status: 500 }
    );
  }
}
