import type {
  IntegrationProvider,
  OrganizationIntegration,
  OrganizationIntegrationListResponse,
  WompiCredentialsRequest,
} from "@/types/organizationIntegrations";

/** Keeps the backend's status, code and field, e.g. 503 INTEGRATIONS_UNAVAILABLE or 400 on private_key. */
export class OrganizationIntegrationError extends Error {
  status: number;
  code?: string;
  field?: string;

  constructor(message: string, status: number, code?: string, field?: string) {
    super(message);
    this.name = "OrganizationIntegrationError";
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/organization/integrations${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new OrganizationIntegrationError("Unauthorized", 401);
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    const message =
      response.status === 403
        ? "No tienes permiso para cambiar esto."
        : body.error || `HTTP error! status: ${response.status}`;
    throw new OrganizationIntegrationError(message, response.status, body.code, body.field);
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export const organizationIntegrationsApi = {
  list(): Promise<OrganizationIntegrationListResponse> {
    return request("");
  },

  putWompi(credentials: WompiCredentialsRequest): Promise<OrganizationIntegration> {
    return request("/wompi", { method: "PUT", body: JSON.stringify(credentials) });
  },

  remove(provider: IntegrationProvider): Promise<void> {
    return request(`/${provider}`, { method: "DELETE" });
  },
};
