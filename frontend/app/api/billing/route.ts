import { serverApiPassthrough } from "@/lib/api/server";

// The admin's subscription: plan, period, due date, next charge, card and charges.
export async function GET() {
  return serverApiPassthrough("/billing");
}
