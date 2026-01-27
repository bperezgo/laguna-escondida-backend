import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  Expense,
  CreateExpenseRequest,
  ExpenseListResponse,
} from "@/types/expense";

// GET /api/expenses - Get all expenses with optional filters
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const params = new URLSearchParams();

  // Forward filter parameters to the backend
  const categoryId = searchParams.get("category_id");
  const supplierId = searchParams.get("supplier_id");
  const startDate = searchParams.get("start_date");
  const endDate = searchParams.get("end_date");

  if (categoryId) params.append("category_id", categoryId);
  if (supplierId) params.append("supplier_id", supplierId);
  if (startDate) params.append("start_date", startDate);
  if (endDate) params.append("end_date", endDate);

  const queryString = params.toString();
  const endpoint = `/expenses${queryString ? `?${queryString}` : ""}`;

  const response = await serverApiRequest<ExpenseListResponse>(endpoint);

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch expenses" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/expenses - Create a new expense
export async function POST(request: NextRequest) {
  try {
    const body: CreateExpenseRequest = await request.json();
    const response = await serverApiRequest<Expense>("/expenses", {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to create expense" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating expense:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create expense",
      },
      { status: 500 }
    );
  }
}
