import type {
  OrganizationSettingsResponse,
  UpdateOrganizationSettingsRequest,
} from "@/types/organizationSettings";

/** Keeps the backend's code and field, e.g. INVALID_ORDERING_SETTINGS on ordering.takeaway.schedule[1]. */
export class OrganizationSettingsError extends Error {
  status: number;
  code?: string;
  field?: string;

  constructor(message: string, status: number, code?: string, field?: string) {
    super(message);
    this.name = "OrganizationSettingsError";
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

async function request(options?: RequestInit): Promise<OrganizationSettingsResponse> {
  const response = await fetch("/api/organization/settings", {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new OrganizationSettingsError("Unauthorized", 401);
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    const message =
      response.status === 403
        ? "No tienes permiso para cambiar esto."
        : body.error || `HTTP error! status: ${response.status}`;
    throw new OrganizationSettingsError(message, response.status, body.code, body.field);
  }
  return response.json();
}

export const organizationSettingsApi = {
  get(): Promise<OrganizationSettingsResponse> {
    return request();
  },

  /** Send only the fields being changed: pause fields need ordering:pause, schedules ordering:manage. */
  update(patch: UpdateOrganizationSettingsRequest): Promise<OrganizationSettingsResponse> {
    return request({ method: "PATCH", body: JSON.stringify(patch) });
  },
};
