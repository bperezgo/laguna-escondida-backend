import { serverApiPassthrough } from "@/lib/api/server";

// Fresh acceptance tokens on every read: they are single use.
export async function GET() {
  return serverApiPassthrough("/billing/card-setup");
}
