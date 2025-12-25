import { serverApiRequest } from "@/lib/api/server";
import { Command } from "@/types/command";

// PATCH /api/commands/:id - Update command status
export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const body = await request.json();

  return serverApiRequest<Command>(`/commands/${id}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}
