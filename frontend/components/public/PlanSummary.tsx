import { Check } from "@phosphor-icons/react/dist/ssr";
import { PLAN_CARDS, PLAN_LABELS, formatPesos, type Plan } from "@/lib/plans";
import styles from "./signup.module.css";

export interface PlanSummaryProps {
  plan: Plan;
  /** Null when prices could not be read: the plan shows without one. */
  monthlyPrice: number | null;
  onChange?: () => void;
  showIncludes?: boolean;
}

export default function PlanSummary({ plan, monthlyPrice, onChange, showIncludes = true }: PlanSummaryProps) {
  const card = PLAN_CARDS.find((c) => c.plan === plan);
  return (
    <>
      <div className={styles.plan}>
        <div>
          <p className={styles.planLabel}>Tu plan</p>
          <p className={styles.planName}>{PLAN_LABELS[plan]}</p>
          {onChange && (
            <button type="button" className={styles.linkButton} onClick={onChange}>
              Cambiar de plan
            </button>
          )}
        </div>
        {monthlyPrice !== null && (
          <p className={styles.price}>
            {formatPesos(monthlyPrice)} <span className={styles.priceUnit}>/ mes</span>
          </p>
        )}
      </div>
      {showIncludes && card && (
        <ul className={styles.includes}>
          {card.includes.map((item) => (
            <li key={item}>
              <Check size={16} weight="bold" aria-hidden />
              <span>{item}</span>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
