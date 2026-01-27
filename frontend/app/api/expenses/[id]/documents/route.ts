import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";

// POST /api/expenses/:id/documents - Upload document for expense
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const searchParams = request.nextUrl.searchParams;
    const categoryCode = searchParams.get("category_code");
    const fileType = searchParams.get("file_type");

    if (!categoryCode) {
      return NextResponse.json(
        { error: "category_code is required" },
        { status: 400 }
      );
    }

    if (!fileType || !["pdf", "xml"].includes(fileType)) {
      return NextResponse.json(
        { error: "file_type must be 'pdf' or 'xml'" },
        { status: 400 }
      );
    }

    // Get the form data from the request
    const formData = await request.formData();
    const file = formData.get("file");

    if (!file || !(file instanceof File)) {
      return NextResponse.json(
        { error: "File is required" },
        { status: 400 }
      );
    }

    // Get the JWT token
    const token = await getAccessToken();

    // Forward the request to the backend
    const backendFormData = new FormData();
    backendFormData.append("file", file);

    const backendUrl = `${config.apiUrl}/expenses/${id}/documents?category_code=${encodeURIComponent(categoryCode)}&file_type=${fileType}`;

    const response = await fetch(backendUrl, {
      method: "POST",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: backendFormData,
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      return NextResponse.json(
        { error: errorData.error || "Failed to upload document" },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error uploading expense document:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to upload document",
      },
      { status: 500 }
    );
  }
}
