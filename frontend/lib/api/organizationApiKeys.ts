import type {
  IssuedOrganizationApiKey,
  OrganizationApiKeyListResponse,
} from "@/types/organizationApiKeys";

/** Keeps the backend's status and field, e.g. 400 on name or 404 for another organization's key. */
export class OrganizationApiKeyError extends Error {
  status: number;
  field?: string;

  constructor(message: string, status: number, field?: string) {
    super(message);
    this.name = "OrganizationApiKeyError";
    this.status = status;
    this.field = field;
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api/organization/api-keys${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new OrganizationApiKeyError("Unauthorized", 401);
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    const message =
      response.status === 403
        ? "No tienes permiso para administrar las claves API."
        : body.error || `HTTP error! status: ${response.status}`;
    throw new OrganizationApiKeyError(message, response.status, body.field);
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export const organizationApiKeysApi = {
  list(): Promise<OrganizationApiKeyListResponse> {
    return request("");
  },

  /** The response is the only time the plaintext is returned. */
  issue(name: string): Promise<IssuedOrganizationApiKey> {
    return request("", { method: "POST", body: JSON.stringify({ name }) });
  },

  revoke(keyId: string): Promise<void> {
    return request(`/${encodeURIComponent(keyId)}`, { method: "DELETE" });
  },
};
