import { serverApiPassthrough } from "@/lib/api/server";

export async function POST() {
  return serverApiPassthrough("/edge/enrollment-codes", { method: "POST" });
}
