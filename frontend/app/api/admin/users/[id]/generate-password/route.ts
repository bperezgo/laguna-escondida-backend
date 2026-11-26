import { serverApiPassthrough } from "@/lib/api/server";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  return serverApiPassthrough(`/admin/users/${encodeURIComponent(id)}/generate-password`, { method: "POST" });
}
