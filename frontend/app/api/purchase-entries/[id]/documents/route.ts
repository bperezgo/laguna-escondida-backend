import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";

// POST /api/purchase-entries/:id/documents - Upload document for purchase entry
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const searchParams = request.nextUrl.searchParams;
    const fileType = searchParams.get("file_type");

    // Get the form data from the request
    const formData = await request.formData();
    const file = formData.get("file");

    if (!file || !(file instanceof File)) {
      return NextResponse.json(
        { error: "File is required" },
        { status: 400 }
      );
    }

    // Check if it's a ZIP file (no file_type required for ZIP)
    const isZipFile = file.type === "application/zip" || 
                      file.type === "application/x-zip-compressed" || 
                      file.name.toLowerCase().endsWith(".zip");

    // For non-ZIP files, file_type is required
    if (!isZipFile && (!fileType || !["pdf", "xml"].includes(fileType))) {
      return NextResponse.json(
        { error: "file_type must be 'pdf' or 'xml' for non-ZIP files" },
        { status: 400 }
      );
    }

    // Get the JWT token
    const token = await getAccessToken();

    // Forward the request to the backend
    const backendFormData = new FormData();
    backendFormData.append("file", file);

    // Build the backend URL - include file_type for all files
    let backendUrl = `${config.apiUrl}/purchase-entries/${id}/documents`;
    if (isZipFile) {
      backendUrl += `?file_type=zip`;
    } else if (fileType) {
      backendUrl += `?file_type=${fileType}`;
    }

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
    console.error("Error uploading purchase entry document:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to upload document",
      },
      { status: 500 }
    );
  }
}
