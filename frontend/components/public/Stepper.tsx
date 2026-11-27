import { Check } from "@phosphor-icons/react/dist/ssr";
import styles from "./Stepper.module.css";

export default function Stepper({ steps, current }: { steps: readonly string[]; current: number }) {
  return (
    <ol className={styles.steps} aria-label="Pasos del registro">
      {steps.map((label, index) => {
        const number = index + 1;
        const state = number < current ? "done" : number === current ? "current" : "todo";
        return (
          <li
            key={label}
            className={`${styles.step} ${state === "done" ? styles.done : state === "current" ? styles.current : ""}`}
            aria-current={state === "current" ? "step" : undefined}
          >
            <span className={styles.dot}>{state === "done" ? <Check size={14} weight="bold" aria-label="Hecho" /> : number}</span>
            {label}
          </li>
        );
      })}
    </ol>
  );
}
