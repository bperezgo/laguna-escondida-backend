import type { EdgeUpdateStatus } from "@/types/edge";

export const UPDATE_POLL_IDLE_MS = 5 * 60_000;
export const UPDATE_POLL_ACTIVE_MS = 10_000;

/** An update this browser asked for, kept until a newer result shows up. */
export interface PendingUpdate {
  version: string;
  /** `last_result.finished_at` when the request was made. */
  baselineFinishedAt: string | null;
}

export type UpdateBannerTone = "info" | "success" | "warning" | "danger";

export type UpdateBannerState =
  | { kind: "available"; version: string; message: string; tone: "info" }
  | { kind: "updating"; version: string; message: string; tone: "info" }
  | {
      kind: "result";
      message: string;
      tone: UpdateBannerTone;
      finishedAt: string;
      /** Version to retry with, when the attempt was refused and is still staged. */
      retryVersion: string | null;
    };

export function availableMessage(version: string): string {
  return `Versión ${version} disponible`;
}

export function updatingMessage(version: string): string {
  return `Actualizando a ${version}… el sistema se reiniciará en unos minutos.`;
}

export function deriveUpdateBanner(
  status: EdgeUpdateStatus | null,
  pending: PendingUpdate | null,
  dismissedFinishedAt: string | null,
): UpdateBannerState | null {
  const lastResult = status?.last_result ?? null;
  const hasNewResult =
    !!lastResult && lastResult.finished_at !== pending?.baselineFinishedAt;

  const activeVersion =
    status?.in_progress?.version ??
    status?.requested?.version ??
    (pending && !hasNewResult ? pending.version : null);
  if (activeVersion) {
    return {
      kind: "updating",
      version: activeVersion,
      message: updatingMessage(activeVersion),
      tone: "info",
    };
  }

  if (!status?.enabled) return null;

  const available = status.available?.version ?? null;

  if (
    lastResult &&
    lastResult.outcome !== "drill_rolled_back" &&
    lastResult.finished_at !== dismissedFinishedAt
  ) {
    const base = {
      kind: "result" as const,
      finishedAt: lastResult.finished_at,
      retryVersion: null,
    };
    switch (lastResult.outcome) {
      case "success":
        return {
          ...base,
          tone: "success",
          message: `Actualizado a ${lastResult.version}`,
        };
      case "refused":
        return {
          ...base,
          tone: "warning",
          message: `No se pudo actualizar: ${lastResult.message ?? ""}`.trim(),
          retryVersion: available,
        };
      case "rolled_back":
        return {
          ...base,
          tone: "warning",
          message: "La actualización falló; se restauró la versión anterior.",
        };
      case "rollback_failed":
        return {
          ...base,
          tone: "danger",
          message: "Error grave en la actualización. Contacte a soporte.",
        };
    }
  }

  if (available) {
    return {
      kind: "available",
      version: available,
      message: availableMessage(available),
      tone: "info",
    };
  }

  return null;
}
