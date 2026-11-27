import { PLAN_CARDS, RECOMMENDED_PLAN, formatPesos, planPrice, type Plan, type PlanPrices } from "@/lib/plans";
import styles from "./signup.module.css";

export interface PlanPickerProps {
  value: Plan;
  prices: PlanPrices | null;
  onChange: (plan: Plan) => void;
  disabled?: boolean;
}

export default function PlanPicker({ value, prices, onChange, disabled = false }: PlanPickerProps) {
  return (
    <fieldset className={styles.options} disabled={disabled}>
      <legend className={styles.planLabel}>Elige tu plan</legend>
      {PLAN_CARDS.map((card) => {
        const price = planPrice(prices, card.plan, "monthly");
        const selected = value === card.plan;
        return (
          <label key={card.plan} className={`${styles.option} ${selected ? styles.selected : ""}`}>
            <span className={styles.optionHead}>
              <input type="radio" name="plan" value={card.plan} checked={selected} onChange={() => onChange(card.plan)} />
              <span className={styles.optionName}>
                {card.title}
                {card.plan === RECOMMENDED_PLAN && <span className={styles.badge}>Recomendado</span>}
              </span>
              {price !== null && (
                <span className={styles.optionPrice}>
                  {formatPesos(price)} <span className={styles.priceUnit}>/ mes</span>
                </span>
              )}
            </span>
            <span className={styles.muted}>{card.includes.join(" · ")}</span>
          </label>
        );
      })}
    </fieldset>
  );
}
