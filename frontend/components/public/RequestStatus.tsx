import { Check } from "@phosphor-icons/react/dist/ssr";
import styles from "./signup.module.css";

const LINES = [
  { label: "Recibimos tu solicitud", state: "done" },
  { label: "Revisamos tu solicitud", state: "current" },
  { label: "Empiezan tus 14 días de prueba", state: "todo" },
] as const;

// Shown instead of the card form while the cloud takes no cards: the operator starts the trial.
export default function RequestStatus({ supportEmail, supportMailto }: { supportEmail: string | null; supportMailto: string | null }) {
  return (
    <section className={styles.card} aria-labelledby="request-status-title">
      <h2 id="request-status-title" className={styles.sectionTitle}>
        Tu solicitud está en revisión
      </h2>
      <ol className={styles.progress}>
        {LINES.map((line) => (
          <li
            key={line.label}
            className={line.state === "done" ? styles.progressDone : line.state === "current" ? styles.progressCurrent : undefined}
            aria-current={line.state === "current" ? "step" : undefined}
          >
            <span className={styles.progressDot}>{line.state === "done" && <Check size={13} weight="bold" aria-label="Hecho" />}</span>
            {line.label}
          </li>
        ))}
      </ol>
      <p className={styles.muted}>
        Cuando aprobemos tu solicitud podrás iniciar sesión con tu cuenta de Google y empezar a usar el sistema.
      </p>
      {supportEmail && supportMailto && (
        <p className={styles.muted}>
          ¿Tienes preguntas? Escríbenos a{" "}
          <a className={styles.link} href={supportMailto}>
            {supportEmail}
          </a>
        </p>
      )}
    </section>
  );
}
