import type { SyncBlocked } from "@/types/edge";

const MESSAGES: Record<SyncBlocked, string> = {
  replaced: "Esta caja fue reemplazada por otra. Ya no sincroniza con la nube.",
  suspended: "La cuenta está suspendida. Esta caja no sincroniza con la nube.",
};

/** The message for an admin or manager; null for anyone else or when nothing blocks the box. */
export function syncBlockedMessage(
  blocked: SyncBlocked | null | undefined,
  canSee: boolean
): string | null {
  if (!blocked || !canSee) return null;
  return MESSAGES[blocked] ?? null;
}
