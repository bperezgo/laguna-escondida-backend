import type { ReactNode } from "react";
import { CheckCircle } from "@phosphor-icons/react/dist/ssr";
import { SIGNUP_STEPS } from "@/lib/signup";
import Stepper from "./Stepper";
import styles from "./PublicLayout.module.css";

export const PRODUCT_NAME = "Kuenta";

const BRAND_POINTS = [
  "14 días de prueba gratis",
  "Órdenes, cocina, caja e inventario en un solo lugar",
  "Se instala en el computador que ya tiene tu restaurante",
];

function Mark({ className = "" }: { className?: string }) {
  return (
    <span className={`${styles.mark} ${className}`}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/favicon.svg" alt="" />
      {PRODUCT_NAME}
    </span>
  );
}

export interface PublicLayoutProps {
  /** `single` has no brand panel: the edge box's sign-in and the invitation page. */
  variant?: "split" | "single";
  /** 1 to 3 on the signup pages; absent elsewhere. */
  step?: 1 | 2 | 3;
  title: ReactNode;
  subtitle?: ReactNode;
  wide?: boolean;
  children: ReactNode;
}

export default function PublicLayout({ variant = "split", step, title, subtitle, wide = false, children }: PublicLayoutProps) {
  return (
    <div className={`${styles.page} ${variant === "split" ? styles.split : styles.single}`}>
      {variant === "split" && (
        <aside className={styles.brand}>
          <Mark />
          <div className={styles.brandCopy}>
            <p className={styles.brandTitle}>El sistema de tu restaurante, listo hoy.</p>
            <ul className={styles.points}>
              {BRAND_POINTS.map((point) => (
                <li key={point}>
                  <CheckCircle size={22} weight="fill" aria-hidden />
                  <span>{point}</span>
                </li>
              ))}
            </ul>
          </div>
          <p className={styles.brandFoot}>Hecho en Colombia para restaurantes.</p>
        </aside>
      )}
      <main className={styles.main}>
        <div className={`${styles.inner} ${wide ? styles.wide : ""}`}>
          <Mark className={styles.mobileMark} />
          {step && <Stepper steps={SIGNUP_STEPS} current={step} />}
          <header className={styles.heading}>
            <h1 className={styles.title}>{title}</h1>
            {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
          </header>
          {children}
        </div>
      </main>
    </div>
  );
}
