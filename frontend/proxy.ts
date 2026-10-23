import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { guardRedirect } from "@/lib/routes/guard";

export function proxy(request: NextRequest) {
  const accessToken = request.cookies.get("access_token");
  const target = guardRedirect(request.nextUrl.pathname, !!accessToken?.value);

  return target
    ? NextResponse.redirect(new URL(target, request.url))
    : NextResponse.next();
}

// Configure which routes should be processed by this middleware
export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * and except files served from public/ (favicon.ico, favicon.svg,
     * logo-navbar.png, …), which the sign-in page needs before a session exists.
     */
    "/((?!api|_next/static|_next/image|.*\\.[^/]+$).*)",
  ],
};
