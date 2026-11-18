import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { UserWithRoles, UpdateUserRequest } from "@/types/user";

// GET /api/admin/users/:id - Get a user by ID
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<UserWithRoles>(`/admin/users/${id}`);

  if (!response.ok) {
    return NextResponse.json(
      { error: "User not found" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}

// PUT /api/admin/users/:id - Update a user (name, roles, active)
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateUserRequest = await request.json();
    const response = await serverApiRequest<UserWithRoles>(`/admin/users/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to update user" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating user:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to update user",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/admin/users/:id - Soft-delete a user
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const response = await serverApiRequest(`/admin/users/${id}`, {
      method: "DELETE",
    });

    if (!response.ok && response.status !== 204) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to delete user" },
        { status: response.status }
      );
    }

    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error deleting user:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to delete user",
      },
      { status: 500 }
    );
  }
}
