import type { UserWithRoles } from "../../types/user.ts";

export const INSTALLER_DOWNLOAD_PATH = "/api/edge/installer";

export function secondsLeft(expiresAt: string, now: number): number {
  const end = Date.parse(expiresAt);
  if (Number.isNaN(end)) return 0;
  return Math.max(0, Math.ceil((end - now) / 1000));
}

export function codeExpired(expiresAt: string, now: number): boolean {
  return secondsLeft(expiresAt, now) === 0;
}

export function formatCountdown(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  const minutes = Math.floor(s / 60);
  return `${minutes}:${String(s % 60).padStart(2, "0")}`;
}

// The box user signup creates: a manager without an email. The oldest one when there are several.
export function findBoxUser(users: UserWithRoles[]): UserWithRoles | null {
  const candidates = users
    .filter((u) => !u.user.email && u.user.active && u.roles.some((r) => r.name === "manager"))
    .sort((a, b) => a.user.created_at.localeCompare(b.user.created_at));
  return candidates[0] ?? null;
}

const CODE_ERRORS: Record<number, string> = {
  403: "Solo un administrador de un restaurante activo puede generar el código.",
  404: "La caja solo se instala desde la nube.",
};

export function installBoxErrorMessage(status: number, message: string): string {
  return CODE_ERRORS[status] ?? (message || "No se pudo completar la acción.");
}

export interface ShownBoxPassword {
  username: string;
  password: string;
}

export type BoxPasswordAction =
  | { type: "generated"; shown: ShownBoxPassword }
  | { type: "hidden" }
  | { type: "left" };

// The password lives only in this state: never in storage or the URL, and it is dropped when
// the admin hides it or leaves the page. Seeing it again means generating a new one.
export function boxPasswordReducer(state: ShownBoxPassword | null, action: BoxPasswordAction): ShownBoxPassword | null {
  switch (action.type) {
    case "generated":
      return action.shown;
    case "hidden":
    case "left":
      return null;
    default:
      return state;
  }
}
