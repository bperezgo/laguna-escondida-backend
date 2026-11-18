// Routes reachable without the access_token cookie. Everything else requires it.
export const publicRoutes = ["/signin"];

function isPublicRoute(pathname: string): boolean {
  return publicRoutes.some(
    (route) => pathname === route || pathname.startsWith(`${route}/`)
  );
}

// Returns where the request should be redirected, or null to let it through.
export function guardRedirect(
  pathname: string,
  isAuthenticated: boolean
): string | null {
  if (pathname === "/") {
    return isAuthenticated ? "/home" : "/signin";
  }

  if (isPublicRoute(pathname)) {
    // Already signed in: the sign-in page has nothing to offer.
    return isAuthenticated ? "/home" : null;
  }

  if (!isAuthenticated) {
    return `/signin?redirect=${encodeURIComponent(pathname)}`;
  }

  return null;
}
