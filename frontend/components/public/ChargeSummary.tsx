import { formatPesos, type Period } from "@/lib/plans";
import { PERIOD_UNITS } from "@/lib/billing/setup";
import styles from "./signup.module.css";

export interface ChargeSummaryProps {
  period: Period;
  /** The backend's amount for the chosen plan and period; null when it could not be read. */
  amount: number | null;
  /** Already written out: "10 de diciembre de 2026". */
  firstChargeOn: string;
  trialDays: number;
}

export default function ChargeSummary({ period, amount, firstChargeOn, trialDays }: ChargeSummaryProps) {
  return (
    <div className={styles.charge} aria-live="polite">
      <p className={styles.today} style={{ margin: 0 }}>
        <span>Hoy pagas</span>
        <strong>{formatPesos(0)}</strong>
      </p>
      <p className={styles.chargeRow} style={{ margin: 0 }}>
        <span>Primer cobro el {firstChargeOn}</span>
        {amount !== null && (
          <strong>
            {formatPesos(amount)} / {PERIOD_UNITS[period]}
          </strong>
        )}
      </p>
      <p className={styles.muted}>Tienes {trialDays} días de prueba gratis. No hacemos ningún cobro antes de esa fecha.</p>
    </div>
  );
}
