import type { BoxSyncState } from "../signIn.ts";
import { syncBlockedMessage } from "./syncBlocked.ts";

export const BOX_WAITING_TITLE = "Esperando a que lleguen los usuarios desde la nube…";

const GUIDANCE: Record<"waiting" | "offline" | "failed", string> = {
  waiting:
    "Los usuarios llegan en uno o dos minutos. Si tarda más, revisa en la nube, en Usuarios, que exista el usuario de la caja.",
  offline: "La caja no tiene conexión a internet. Revisa la red; los usuarios llegarán cuando vuelva.",
  failed:
    "La caja no pudo recibir los datos de la nube. Contacta a soporte; los registros están en C:\\kuentapos\\logs.",
};

/** What to check on a box that has no user yet, by how its last pull went. */
export function boxWaitingGuidance(sync: BoxSyncState | undefined): string {
  if (sync === "replaced" || sync === "suspended") return syncBlockedMessage(sync, true) ?? GUIDANCE.waiting;
  return GUIDANCE[sync ?? "waiting"];
}
