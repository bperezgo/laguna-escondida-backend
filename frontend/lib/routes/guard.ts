// Routes reachable without the access_token cookie. Everything else requires it.
// A signed-in user is sent home from these.
// /recuperar is where the admin of a stopped account pays its way back, with a setup session and no sign-in.
export const publicRoutes = ["/signin", "/registro", "/recuperar"];

// Reachable with or without a session: an invitation link opened in a signed-in browser still
// shows the invitation.
export const openRoutes = ["/invitacion"];

// The last signup step is shown to the person the card just signed in, so it needs a session
// although it lives under a public route.
export const signedInSignupRoutes = ["/registro/listo"];

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

  if (matches(signedInSignupRoutes, pathname)) {
    return isAuthenticated ? null : "/signin";
  }

  if (matches(publicRoutes, pathname)) {
    return isAuthenticated ? "/home" : null;
  }

  if (!isAuthenticated) {
    return `/signin?redirect=${encodeURIComponent(pathname)}`;
  }

  return null;
}
