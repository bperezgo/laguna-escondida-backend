import { apiRequest } from "./config";
import type { Command } from "@/types/command";

/**
 * Mark a command as completed
 */
export async function completeCommand(commandId: string): Promise<Command> {
  return apiRequest<Command>(`/commands/${commandId}`, {
    method: "PATCH",
    body: JSON.stringify({ status: "completed" }),
  });
}
