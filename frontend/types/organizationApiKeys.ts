/** A key as listed: never its plaintext or hash. */
export interface OrganizationApiKey {
  id: string;
  organization_id: string;
  user_id: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

/** The only response that carries the plaintext, shown once. */
export interface IssuedOrganizationApiKey extends OrganizationApiKey {
  key: string;
}

export interface OrganizationApiKeyListResponse {
  api_keys: OrganizationApiKey[];
}

export interface IssueOrganizationApiKeyRequest {
  name: string;
}
