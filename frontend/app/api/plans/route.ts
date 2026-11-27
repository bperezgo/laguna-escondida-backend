import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";

// GET /api/plans - Public: the price of each plan. An edge box has no prices and answers 404.
export async function GET() {
  try {
    const response = await fetch(`${config.apiUrl}/plans`, { cache: "no-store" });
    if (!response.ok) {
      return NextResponse.json({ error: "Plan prices unavailable" }, { status: response.status });
    }
    return NextResponse.json(await response.json());
  } catch (error) {
    console.error("Error loading plan prices:", error);
    return NextResponse.json({ error: "Plan prices unavailable" }, { status: 502 });
  }
}
