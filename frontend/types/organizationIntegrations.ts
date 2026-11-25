export type IntegrationProvider = "wompi" | "marketing_app";
export type IntegrationEnvironment = "sandbox" | "production";

/** Never carries a secret: the backend returns only the last 4 characters of each. */
export interface WompiIntegration {
  provider: "wompi";
  environment: IntegrationEnvironment;
  private_key_hint: string;
  events_secret_hint: string;
  /** The QR keys: set together or not at all. */
  public_key_hint?: string;
  integrity_secret_hint?: string;
  /** True only when both QR keys are set; without them orders are paid with the link only. */
  qr_enabled?: boolean;
  webhook_path: string;
  created_at: string;
  updated_at: string;
}

export interface MarketingAppIntegration {
  provider: "marketing_app";
  signing_secret_hint: string;
  created_at: string;
  updated_at: string;
}

export type OrganizationIntegration = WompiIntegration | MarketingAppIntegration;

export interface OrganizationIntegrationListResponse {
  integrations: OrganizationIntegration[];
}

export interface WompiCredentialsRequest {
  private_key: string;
  events_secret: string;
  public_key?: string;
  integrity_secret?: string;
}

/** The only response that carries a secret: the one just generated, shown once. */
export interface GeneratedMarketingAppSecret extends MarketingAppIntegration {
  signing_secret: string;
}
