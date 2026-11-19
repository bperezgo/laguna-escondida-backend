export type IntegrationProvider = "wompi";
export type IntegrationEnvironment = "sandbox" | "production";

/** Never carries a secret: the backend returns only the last 4 characters of each. */
export interface OrganizationIntegration {
  provider: IntegrationProvider;
  environment: IntegrationEnvironment;
  private_key_hint: string;
  events_secret_hint: string;
  webhook_path: string;
  created_at: string;
  updated_at: string;
}

export interface OrganizationIntegrationListResponse {
  integrations: OrganizationIntegration[];
}

export interface WompiCredentialsRequest {
  private_key: string;
  events_secret: string;
}
