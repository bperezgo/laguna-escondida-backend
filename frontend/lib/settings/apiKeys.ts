import { PERMISSIONS } from "../permissions/constants.ts";
import type { IssuedOrganizationApiKey, OrganizationApiKey } from "../../types/organizationApiKeys.ts";

export const API_KEY_MAX_NAME_LENGTH = 255;

export interface ApiKeysLoad {
  status: number;
  keys?: OrganizationApiKey[];
}

export type ApiKeysCardState = { kind: "error"; message: string } | { kind: "loaded"; keys: OrganizationApiKey[] };

export function apiKeysCardState(load: ApiKeysLoad): ApiKeysCardState {
  if (load.status === 403) return { kind: "error", message: "No tienes permiso para ver las claves API." };
  if (load.status === 404) return { kind: "error", message: "Las claves API solo se administran en la nube." };
  if (load.status < 200 || load.status >= 300 || !load.keys) {
    return { kind: "error", message: "Error al cargar las claves API." };
  }
  return { kind: "loaded", keys: load.keys };
}

/** Held only while the dialog is open: never in the list, the URL, storage or logs. */
export interface ShownApiKey {
  name: string;
  key: string;
  mcpUrl: string | null;
}

export function apiKeyIssued(
  keys: OrganizationApiKey[],
  issued: IssuedOrganizationApiKey,
  mcpUrl: string | null
): { keys: OrganizationApiKey[]; shown: ShownApiKey } {
  const { key, ...listed } = issued;
  return { keys: [listed, ...keys.filter((k) => k.id !== listed.id)], shown: { name: listed.name, key, mcpUrl } };
}

export function apiKeyRevoked(keys: OrganizationApiKey[], keyId: string, at: string): OrganizationApiKey[] {
  return keys.map((k) => (k.id === keyId && !k.revoked_at ? { ...k, revoked_at: at } : k));
}

export function canRevokeApiKey(key: OrganizationApiKey): boolean {
  return key.revoked_at === null;
}

export function validateApiKeyName(raw: string): string {
  const name = raw.trim();
  if (!name) return "Escribe un nombre para la clave.";
  if (Array.from(name).length > API_KEY_MAX_NAME_LENGTH) return `El nombre admite hasta ${API_KEY_MAX_NAME_LENGTH} caracteres.`;
  return "";
}

export function apiKeyErrorMessage(status: number, message: string): string {
  if (status === 403) return "No tienes permiso para administrar las claves API.";
  if (status === 404) return "La clave ya no existe en este restaurante.";
  return message || "Error al administrar las claves API.";
}

/** Unset or blank means the page shows no MCP URL. */
export function mcpPublicUrl(raw: string | undefined): string | null {
  const url = raw?.trim();
  return url ? url : null;
}

export function showApiKeysCard(hasPermission: (permission: string) => boolean): boolean {
  return hasPermission(PERMISSIONS.API_KEYS_MANAGE);
}

export const API_KEY_SHOWN_ONCE_NOTE =
  "Cópiala ahora y guárdala en la configuración de la consola o del agente. No se volverá a mostrar.";

export const API_KEY_REVOKE_WARNING =
  "La clave deja de funcionar de inmediato y no se puede recuperar. Si un agente o consola la usa, perderá el acceso hasta que le pongas una clave nueva.";
