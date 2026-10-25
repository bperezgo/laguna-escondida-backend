import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { LEGACY_EDGE_CONTEXT, parseSignInContext } from "@/lib/signIn";

// GET /api/auth/context - Public: which sign-in form to show (cloud: email, edge: username)
export async function GET() {
  try {
    const response = await fetch(`${config.apiUrl}/auth/context`, { cache: "no-store" });

    if (response.status === 404) {
      return NextResponse.json(LEGACY_EDGE_CONTEXT);
    }
    if (!response.ok) {
      return NextResponse.json(
        { error: "Failed to load the sign-in context" },
        { status: response.status }
      );
    }

    return NextResponse.json(parseSignInContext(await response.json()));
  } catch (error) {
    console.error("Error loading the sign-in context:", error);
    return NextResponse.json(
      { error: "Failed to load the sign-in context" },
      { status: 502 }
    );
  }
}
