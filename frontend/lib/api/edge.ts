import { apiRequest } from "./config";
import type { EdgeStatus } from "@/types/edge";

export const edgeApi = {
  /**
   * Get the edge node's connectivity & sync status.
   */
  async getStatus(): Promise<EdgeStatus> {
    return apiRequest<EdgeStatus>("/edge/status");
  },
};
