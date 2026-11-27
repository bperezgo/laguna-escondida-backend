import { serverApiPassthrough } from "@/lib/api/server";

export async function POST() {
  return serverApiPassthrough("/billing/cancel", { method: "POST" });
}
