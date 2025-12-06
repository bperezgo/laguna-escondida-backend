import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { NextResponse } from "next/server";
import { cookies } from "next/headers";

/**
 * Server-side API client that calls the Golang backend directly
 * This should only be used in Next.js API routes and server components
 * Automatically includes the JWT token from cookies in the Authorization header
 */
export async function serverApiRequest<T>(
  endpoint: string,
  options?: RequestInit
): Promise<NextResponse<T>> {
  const url = `${config.apiUrl}${endpoint}`;

  // Get the JWT token from cookies
  const token = await getAccessToken();

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options?.headers as Record<string, string>),
  };

  // Add Authorization header if token exists
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const response = await fetch(url, {
    ...options,
    headers,
  });

  // Handle unauthorized responses by clearing the invalid token
  if (response.status === 401) {
    const cookieStore = await cookies();
    cookieStore.delete("access_token");
  }

  if (response.status === 204) {
    return NextResponse.json({} as T, { status: 204 });
  }

  return NextResponse.json(await response.json(), {
    status: response.status,
  });
}
