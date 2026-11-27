import { PERIODS, formatPesos, planPrice, type Period, type Plan, type PlanPrices } from "@/lib/plans";
import { PERIOD_LABELS, PERIOD_UNITS } from "@/lib/billing/setup";
import styles from "./signup.module.css";

export interface PeriodToggleProps {
  plan: Plan;
  value: Period;
  prices: PlanPrices | null;
  onChange: (period: Period) => void;
  disabled?: boolean;
}

export default function PeriodToggle({ plan, value, prices, onChange, disabled = false }: PeriodToggleProps) {
  return (
    <fieldset className={styles.periods} disabled={disabled}>
      <legend className={styles.planLabel}>Cómo quieres pagar</legend>
      {PERIODS.map((period) => {
        const price = planPrice(prices, plan, period);
        const selected = value === period;
        return (
          <label key={period} className={`${styles.period} ${selected ? styles.selected : ""}`}>
            <span className={styles.periodName}>
              <input type="radio" name="period" value={period} checked={selected} onChange={() => onChange(period)} />
              {PERIOD_LABELS[period]}
              {period === "annual" && <span className={styles.saving}>2 meses gratis</span>}
            </span>
            {price !== null && (
              <span className={styles.muted}>
                {formatPesos(price)} / {PERIOD_UNITS[period]}
              </span>
            )}
          </label>
        );
      })}
    </fieldset>
  );
}
