import { apiRequest } from "./config";
import type { EdgeStatus, EdgeUpdateStatus } from "@/types/edge";

export const edgeApi = {
  /**
   * Get the edge node's connectivity & sync status.
   */
  async getStatus(): Promise<EdgeStatus> {
    return apiRequest<EdgeStatus>("/edge/status");
  },

  async getUpdate(): Promise<EdgeUpdateStatus> {
    return apiRequest<EdgeUpdateStatus>("/edge/update");
  },

  async requestUpdate(version: string): Promise<void> {
    await apiRequest<unknown>("/edge/update", {
      method: "POST",
      body: JSON.stringify({ version }),
    });
  },
};
