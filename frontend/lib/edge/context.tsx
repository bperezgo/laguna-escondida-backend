"use client";

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  ReactNode,
} from "react";
import { edgeApi } from "@/lib/api/edge";
import type { EdgeStatus } from "@/types/edge";

// Lightweight poll. Start at 15s; adjust if the backend exposes SSE for this.
const POLL_INTERVAL_MS = 15_000;

interface EdgeStatusContextValue {
  status: EdgeStatus | null;
  /** True when the node has no internet, or the status endpoint is unreachable. */
  isOffline: boolean;
  /** True until the first poll resolves. */
  isLoading: boolean;
  /** unsynced_orders + pending_invoices (0 when unknown). */
  pendingSyncCount: number;
  refresh: () => Promise<void>;
}

const EdgeStatusContext = createContext<EdgeStatusContextValue | undefined>(
  undefined
);

interface EdgeStatusProviderProps {
  children: ReactNode;
}

export function EdgeStatusProvider({ children }: EdgeStatusProviderProps) {
  const [status, setStatus] = useState<EdgeStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [hasError, setHasError] = useState(false);

  const fetchStatus = useCallback(async () => {
    try {
      const data = await edgeApi.getStatus();
      setStatus(data);
      setHasError(false);
    } catch (err) {
      // Fail soft: an unreachable edge/cloud node means we're offline, not broken.
      console.error("Error fetching edge status:", err);
      setHasError(true);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // While the very first poll is in flight we keep isOffline false to avoid
  // flashing "Sin conexión" before we actually know.
  const isOffline = hasError || (status !== null && !status.online);
  const pendingSyncCount =
    (status?.unsynced_orders ?? 0) + (status?.pending_invoices ?? 0);

  const value: EdgeStatusContextValue = {
    status,
    isOffline,
    isLoading,
    pendingSyncCount,
    refresh: fetchStatus,
  };

  return (
    <EdgeStatusContext.Provider value={value}>
      {children}
    </EdgeStatusContext.Provider>
  );
}

export function useEdgeStatus(): EdgeStatusContextValue {
  const context = useContext(EdgeStatusContext);
  if (context === undefined) {
    throw new Error("useEdgeStatus must be used within an EdgeStatusProvider");
  }
  return context;
}
