// Routes reachable without the access_token cookie. Everything else requires it.
// A signed-in user is sent home from these.
export const publicRoutes = ["/signin", "/registro"];

// Reachable with or without a session: an invitation link opened in a signed-in browser still
// shows the invitation.
export const openRoutes = ["/invitacion"];

function matches(routes: string[], pathname: string): boolean {
  return routes.some(
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

  if (matches(openRoutes, pathname)) return null;

  if (matches(publicRoutes, pathname)) {
    return isAuthenticated ? "/home" : null;
  }

  if (!isAuthenticated) {
    return `/signin?redirect=${encodeURIComponent(pathname)}`;
  }

  return null;
}
