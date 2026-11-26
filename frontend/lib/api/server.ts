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

  // 204 No Content cannot have a body, return empty response
  if (response.status === 204) {
    return new NextResponse(null, { status: 204 }) as NextResponse<T>;
  }

  return NextResponse.json(await response.json(), {
    status: response.status,
  });
}

/**
 * Forwards the backend's status and body unchanged (error `code`s included), never cached:
 * for responses that carry a secret shown once (links, codes, passwords).
 */
export async function serverApiPassthrough(
  endpoint: string,
  options?: RequestInit
): Promise<NextResponse> {
  const headers = { "Cache-Control": "no-store" };
  try {
    const response = await serverApiRequest<unknown>(endpoint, options);
    if (response.status === 204) return new NextResponse(null, { status: 204, headers });
    return NextResponse.json(await response.json(), { status: response.status, headers });
  } catch (error) {
    console.error(`Request to ${endpoint} failed:`, error);
    return NextResponse.json({ error: "Backend unavailable" }, { status: 502, headers });
  }
}

/** A route's own error message, keeping a plan refusal's code so the page can name the plan. */
export async function fallbackErrorResponse(response: Response, fallback: string): Promise<NextResponse> {
  const data = await response.json().catch(() => ({}));
  const body =
    data.code === "plan_required"
      ? { error: fallback, code: data.code, required_plan: data.required_plan }
      : { error: fallback };
  return NextResponse.json(body, { status: response.status });
}
