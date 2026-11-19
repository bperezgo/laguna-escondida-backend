import { NextResponse } from "next/server";

// Passes the backend's error, code, field and required permission through. The request body
// holds secrets, so these routes never log it nor an error raised while reading it.
export async function integrationErrorResponse(response: Response, fallback: string) {
  const errorData = await response.json().catch(() => ({}));
  return NextResponse.json(
    {
      error: errorData.error || fallback,
      code: errorData.code,
      field: errorData.field,
      required: errorData.required,
    },
    { status: response.status }
  );
}
