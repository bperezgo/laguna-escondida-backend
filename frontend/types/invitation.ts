import type { Role } from "./user";

export interface Invitation {
  id: string;
  email: string;
  name: string;
  role_ids: number[];
  roles: Role[];
  invited_by: string;
  invited_by_name: string;
  expires_at: string;
  created_at: string;
}

/** Create and new-link responses: the link is shown once. */
export interface IssuedInvitation {
  invitation: Invitation;
  link: string;
}

export interface InvitationListResponse {
  invitations: Invitation[];
}

export interface CreateInvitationRequest {
  email: string;
  name: string;
  role_ids: number[];
}

/** GET /api/invitations/:token */
export interface InvitationLookup {
  organization_name: string;
  email: string;
  name: string;
  roles: Role[];
  expires_at: string;
}
