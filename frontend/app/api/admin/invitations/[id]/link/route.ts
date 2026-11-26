import { serverApiPassthrough } from "@/lib/api/server";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  return serverApiPassthrough(`/admin/invitations/${encodeURIComponent(id)}/link`, { method: "POST" });
}
