// The organization (restaurant) slug the user signs in to. The cloud requires it; an edge box
// signs in to its own organization when it is left empty.

const STORAGE_KEY = "laguna.signin.organization";

export function normalizeOrganizationSlug(value: string): string {
  return value.trim().toLowerCase();
}

export function readRememberedOrganization(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(STORAGE_KEY) ?? "";
  } catch {
    return "";
  }
}

export function rememberOrganization(slug: string) {
  try {
    if (slug) window.localStorage.setItem(STORAGE_KEY, slug);
  } catch {
    // Storage blocked: the user types the restaurant again next time.
  }
}

// The backend sign-in body: `organization` is omitted when empty so the edge uses its own.
export function signInBody(organization: string, username: string, password: string) {
  const slug = normalizeOrganizationSlug(organization);
  return slug ? { organization: slug, username, password } : { username, password };
}
