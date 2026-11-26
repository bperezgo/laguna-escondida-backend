import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";

// The browser never reaches the backend directly (NEXT_PUBLIC_API_URL is server-side), so the
// download link points here and gets the backend's redirect to the GitHub release asset.
export async function GET() {
  try {
    const response = await fetch(`${config.apiUrl}/public/edge/installer`, {
      redirect: "manual",
      cache: "no-store",
    });
    const location = response.headers.get("location");
    if (response.status >= 300 && response.status < 400 && location) {
      return NextResponse.redirect(location, { status: 302, headers: { "Cache-Control": "no-store" } });
    }
    const body = await response.json().catch(() => ({ error: "installer_unavailable" }));
    const headers: Record<string, string> = { "Cache-Control": "no-store" };
    const retryAfter = response.headers.get("retry-after");
    if (retryAfter) headers["Retry-After"] = retryAfter;
    return NextResponse.json(body, { status: response.ok ? 502 : response.status, headers });
  } catch (error) {
    console.error("Installer lookup failed:", error);
    return NextResponse.json({ error: "installer_unavailable" }, { status: 502 });
  }
}
