import { apiRequest } from "./config";
import type {
  CreateInvitationRequest,
  Invitation,
  InvitationListResponse,
  IssuedInvitation,
} from "@/types/invitation";

export const invitationsApi = {
  async list(): Promise<Invitation[]> {
    const response = await apiRequest<InvitationListResponse>("/admin/invitations");
    return response.invitations ?? [];
  },

  create(data: CreateInvitationRequest): Promise<IssuedInvitation> {
    return apiRequest<IssuedInvitation>("/admin/invitations", { method: "POST", body: JSON.stringify(data) });
  },

  newLink(id: string): Promise<IssuedInvitation> {
    return apiRequest<IssuedInvitation>(`/admin/invitations/${encodeURIComponent(id)}/link`, { method: "POST" });
  },

  async revoke(id: string): Promise<void> {
    await apiRequest(`/admin/invitations/${encodeURIComponent(id)}`, { method: "DELETE" });
  },
};
