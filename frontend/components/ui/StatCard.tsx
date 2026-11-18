import { ReactNode } from "react";
import styles from "./StatCard.module.css";

export interface StatCardProps {
  label: ReactNode;
  value: ReactNode;
  delta?: ReactNode;
  deltaTone?: "up" | "down" | "neutral";
  icon?: ReactNode;
}

export default function StatCard({
  label,
  value,
  delta,
  deltaTone = "neutral",
  icon,
}: StatCardProps) {
  return (
    <div className={styles.card}>
      <div className={styles.top}>
        <span className={styles.label}>{label}</span>
        {icon && <span className={styles.icon}>{icon}</span>}
      </div>
      <div className={styles.value}>{value}</div>
      {delta && <div className={[styles.delta, styles[deltaTone]].join(" ")}>{delta}</div>}
    </div>
  );
}
