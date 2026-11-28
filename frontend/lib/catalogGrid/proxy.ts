export interface ForwardedResponse {
  status: number;
  body: string;
  contentType: string;
}

export type FetchLike = (url: string, init: RequestInit) => Promise<Response>;

// The body goes through as text and the answer comes back as text, so a 413 or 403 that is not
// JSON keeps its status instead of turning into a 500 on a failed parse.
export async function forwardCatalogImport(
  fetchFn: FetchLike,
  url: string,
  token: string | undefined,
  body: string,
): Promise<ForwardedResponse> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetchFn(url, { method: "POST", headers, body });
  return {
    status: response.status,
    body: await response.text(),
    contentType: response.headers.get("Content-Type") ?? "application/json",
  };
}
