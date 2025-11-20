import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Define public routes that don't require authentication
const publicRoutes = ["/signin"];

// Define the protected routes
const protectedRoutes = [
  "/home",
  "/orders",
  "/products",
  "/stock",
  "/invoices",
];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessToken = request.cookies.get("access_token");
  const isAuthenticated = !!accessToken?.value;

  // Handle root path
  if (pathname === "/") {
    if (isAuthenticated) {
      return NextResponse.redirect(new URL("/home", request.url));
    } else {
      return NextResponse.redirect(new URL("/signin", request.url));
    }
  }

  // If user is on signin page and already authenticated, redirect to home
  if (pathname === "/signin" && isAuthenticated) {
    return NextResponse.redirect(new URL("/home", request.url));
  }

  // Check if the current route is protected
  const isProtectedRoute = protectedRoutes.some((route) =>
    pathname.startsWith(route)
  );

  // If it's a protected route and user is not authenticated, redirect to signin
  if (isProtectedRoute && !isAuthenticated) {
    const signInUrl = new URL("/signin", request.url);
    signInUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(signInUrl);
  }

  return NextResponse.next();
}

// Configure which routes should be processed by this middleware
export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     */
    "/((?!api|_next/static|_next/image|favicon.ico).*)",
  ],
};
