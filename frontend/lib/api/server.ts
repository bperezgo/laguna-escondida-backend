import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { NextResponse } from "next/server";

/**
 * Server-side API client that calls the Golang backend directly
 * This should only be used in Next.js API routes and server components
 * Automatically includes the JWT token from cookies in the Authorization header
 */
export async function serverApiRequest<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
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

  if (!response.ok) {
    const error = await response
      .json()
      .catch(() => ({ message: response.statusText }));
    throw new Error(error.message || `HTTP error! status: ${response.status}`);
  }

  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}
