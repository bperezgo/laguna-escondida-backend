export interface ApiErrorBody {
  error?: string;
  message?: string;
  code?: string;
  order_type?: string;
  paused_reason?: string | null;
  next_opens_at?: string | null;
}

/** Keeps the backend's status and body so callers can branch on `code`. */
export class ApiError extends Error {
  status: number;
  body: ApiErrorBody;

  constructor(message: string, status: number, body: ApiErrorBody) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

/**
 * Client-side API request function that calls Next.js API routes
 * These routes then proxy to the Golang backend
 */
export async function apiRequest<T>(
  endpoint: string,
  options?: RequestInit,
): Promise<T> {
  const url = `/api${endpoint}`;

  const response = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  // Handle unauthorized responses by redirecting to signin
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new Error("Unauthorized");
  }

  if (!response.ok) {
    const error: ApiErrorBody = await response
      .json()
      .catch(() => ({ message: response.statusText }));
    throw new ApiError(
      error.error || error.message || `HTTP error! status: ${response.status}`,
      response.status,
      error,
    );
  }

  // 204 No Content has no body, return undefined
  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}
