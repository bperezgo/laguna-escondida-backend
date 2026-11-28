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

// A user without an email can only sign in on the box: the cloud requires an email for every cloud role.
export function boxUsers(users: UserWithRoles[]): UserWithRoles[] {
  return users
    .filter((u) => !u.user.email && u.user.active)
    .sort((a, b) => a.user.username.localeCompare(b.user.username));
}

const CODE_ERRORS: Record<number, string> = {
  403: "Solo un administrador de un restaurante activo puede generar el código.",
  404: "La caja solo se instala desde la nube.",
};

export function installBoxErrorMessage(status: number, message: string): string {
  return CODE_ERRORS[status] ?? (message || "No se pudo completar la acción.");
}
