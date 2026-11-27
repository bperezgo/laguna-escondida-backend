import { serverApiPassthrough } from "@/lib/api/server";

export async function POST() {
  return serverApiPassthrough("/billing/resume", { method: "POST" });
}
