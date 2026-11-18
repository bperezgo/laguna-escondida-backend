import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  UserWithRoles,
  CreateUserRequest,
  UsersListResponse,
} from "@/types/user";

// GET /api/admin/users - List all users with roles
export async function GET() {
  const response = await serverApiRequest<UsersListResponse>("/admin/users");

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch users" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/admin/users - Create a new user
export async function POST(request: NextRequest) {
  try {
    const body: CreateUserRequest = await request.json();
    const response = await serverApiRequest<UserWithRoles>("/admin/users", {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to create user" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating user:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to create user",
      },
      { status: 500 }
    );
  }
}
