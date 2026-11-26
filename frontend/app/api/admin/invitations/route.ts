import { NextRequest } from "next/server";
import { serverApiPassthrough } from "@/lib/api/server";

export async function GET() {
  return serverApiPassthrough("/admin/invitations");
}

export async function POST(request: NextRequest) {
  const body = await request.json().catch(() => ({}));
  return serverApiPassthrough("/admin/invitations", { method: "POST", body: JSON.stringify(body) });
}
