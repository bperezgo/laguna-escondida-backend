import { ReactNode } from "react";
import styles from "./Badge.module.css";

export type BadgeTone = "success" | "warning" | "danger" | "neutral" | "info";

export interface BadgeProps {
  tone?: BadgeTone;
  dot?: boolean;
  children: ReactNode;
  className?: string;
}

export default function Badge({
  tone = "neutral",
  dot = true,
  children,
  className = "",
}: BadgeProps) {
  const cls = [styles.badge, styles[tone], className].filter(Boolean).join(" ");
  return (
    <span className={cls}>
      {dot && <span className={styles.dot} />}
      {children}
    </span>
  );
}

/**
 * Maps common Spanish document/order statuses to a badge tone.
 * Falls back to "neutral" for unknown values.
 */
export function statusToTone(status: string): BadgeTone {
  const s = status.trim().toLowerCase();
  if (["pagado", "pagada", "activo", "activa", "aprobado", "completado", "listo"].includes(s))
    return "success";
  if (["pendiente", "en proceso", "borrador", "abierta", "abierto"].includes(s))
    return "warning";
  if (["anulado", "anulada", "rechazado", "vencido", "cancelado", "cancelada"].includes(s))
    return "danger";
  return "neutral";
}
