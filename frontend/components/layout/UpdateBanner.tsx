"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { edgeApi } from "@/lib/api/edge";
import {
  deriveUpdateBanner,
  UPDATE_POLL_ACTIVE_MS,
  UPDATE_POLL_IDLE_MS,
  type PendingUpdate,
} from "@/lib/edge/update";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { Button, Modal } from "@/components/ui";
import type { EdgeUpdateStatus } from "@/types/edge";
import styles from "./UpdateBanner.module.css";

const DISMISSED_KEY = "laguna.updateBanner.dismissedFinishedAt";

function readDismissed(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(DISMISSED_KEY);
  } catch {
    return null;
  }
}

function writeDismissed(finishedAt: string) {
  try {
    window.localStorage.setItem(DISMISSED_KEY, finishedAt);
  } catch {
    // Storage blocked: the dismissal lasts until the next reload.
  }
}

export default function UpdateBanner() {
  const { hasPermission, isEdge, isLoading } = usePermissions();
  const allowed = !isLoading && isEdge && hasPermission(PERMISSIONS.EDGE_UPDATE);

  const [status, setStatus] = useState<EdgeUpdateStatus | null>(null);
  const [pending, setPending] = useState<PendingUpdate | null>(null);
  // Safe to read on first render: nothing shows until permissions load client-side.
  const [dismissed, setDismissed] = useState<string | null>(readDismissed);
  const [confirmVersion, setConfirmVersion] = useState<string | null>(null);
  const [isRequesting, setIsRequesting] = useState(false);
  const [requestError, setRequestError] = useState<string | null>(null);

  const banner = deriveUpdateBanner(status, pending, dismissed);
  const isUpdating = banner?.kind === "updating";
  const isUpdatingRef = useRef(isUpdating);
  useEffect(() => {
    isUpdatingRef.current = isUpdating;
  }, [isUpdating]);

  const fetchStatus = useCallback(async () => {
    try {
      const data = await edgeApi.getUpdate();
      setStatus(data);
      setPending((prev) =>
        prev &&
        (data.last_result?.finished_at ?? null) !== prev.baselineFinishedAt
          ? null
          : prev
      );
    } catch {
      // While updating, edge-node is down by design: keep the last state.
      if (!isUpdatingRef.current) setStatus(null);
    }
  }, []);

  useEffect(() => {
    if (!allowed) return;
    fetchStatus();
    const delay = isUpdating ? UPDATE_POLL_ACTIVE_MS : UPDATE_POLL_IDLE_MS;
    const interval = setInterval(fetchStatus, delay);
    return () => clearInterval(interval);
  }, [allowed, isUpdating, fetchStatus]);

  const closeConfirm = useCallback(() => {
    if (isRequesting) return;
    setConfirmVersion(null);
    setRequestError(null);
  }, [isRequesting]);

  const confirmUpdate = async () => {
    if (!confirmVersion) return;
    setIsRequesting(true);
    setRequestError(null);
    try {
      await edgeApi.requestUpdate(confirmVersion);
      setPending({
        version: confirmVersion,
        baselineFinishedAt: status?.last_result?.finished_at ?? null,
      });
      setConfirmVersion(null);
    } catch (err) {
      setRequestError(err instanceof Error ? err.message : "Error desconocido");
      fetchStatus();
    } finally {
      setIsRequesting(false);
    }
  };

  const dismiss = (finishedAt: string) => {
    writeDismissed(finishedAt);
    setDismissed(finishedAt);
  };

  if (!allowed || !banner) return null;

  const updateVersion =
    banner.kind === "available"
      ? banner.version
      : banner.kind === "result"
        ? banner.retryVersion
        : null;

  return (
    <>
      <div className={`${styles.banner} ${styles[banner.tone]}`} role="status">
        <span>{banner.message}</span>
        <div className={styles.actions}>
          {updateVersion && (
            <Button size="sm" onClick={() => setConfirmVersion(updateVersion)}>
              {banner.kind === "result" ? "Reintentar" : "Actualizar ahora"}
            </Button>
          )}
          {banner.kind === "result" && (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => dismiss(banner.finishedAt)}
            >
              Cerrar
            </Button>
          )}
        </div>
      </div>

      <Modal
        open={confirmVersion !== null}
        onClose={closeConfirm}
        title={`Actualizar a ${confirmVersion ?? ""}`}
        size="sm"
        closeOnOverlay={!isRequesting}
        footer={
          <>
            <Button
              variant="secondary"
              onClick={closeConfirm}
              disabled={isRequesting}
            >
              Cancelar
            </Button>
            <Button onClick={confirmUpdate} disabled={isRequesting}>
              {isRequesting ? "Procesando..." : "Actualizar ahora"}
            </Button>
          </>
        }
      >
        <p>
          El POS no estará disponible durante unos minutos. Hazlo cuando no
          haya órdenes abiertas en curso.
        </p>
        {requestError && (
          <p className={styles.error}>
            No se pudo solicitar la actualización: {requestError}
          </p>
        )}
      </Modal>
    </>
  );
}
