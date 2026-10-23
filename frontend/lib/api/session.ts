// An EventSource cannot see the HTTP status of a failed connection, so a
// stream that errors asks /api/auth/me whether the session is still valid. A
// rejected session (expired, or a token from before multi-tenancy) goes to
// /signin instead of reconnecting forever. Returns true when it redirected.
export async function redirectIfSessionEnded(): Promise<boolean> {
  try {
    const response = await fetch("/api/auth/me");
    if (response.status !== 401) return false;
  } catch {
    // Network down: keep reconnecting.
    return false;
  }
  window.location.href = `/signin?redirect=${encodeURIComponent(window.location.pathname)}`;
  return true;
}
