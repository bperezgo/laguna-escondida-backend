import { serverApiPassthrough } from "@/lib/api/server";

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  return serverApiPassthrough(`/admin/invitations/${encodeURIComponent(id)}`, { method: "DELETE" });
}
