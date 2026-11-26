import type { ReactNode } from "react";

export type NoticeTone = "danger" | "warning";

const tones: Record<NoticeTone, { background: string; border: string; color: string }> = {
  danger: { background: "var(--color-danger-light)", border: "var(--color-danger)", color: "var(--color-danger)" },
  warning: { background: "var(--color-warning-bg)", border: "var(--color-warning-text)", color: "var(--color-warning-text)" },
};

export default function NoticeMessage({
  message,
  mailto,
  linkLabel = "Escríbenos",
  tone = "danger",
  children,
}: {
  message: string;
  mailto?: string | null;
  linkLabel?: string;
  tone?: NoticeTone;
  children?: ReactNode;
}) {
  const t = tones[tone];
  return (
    <div
      role="alert"
      style={{
        borderRadius: "var(--radius-md)",
        backgroundColor: t.background,
        border: `1px solid ${t.border}`,
        color: t.color,
        padding: "0.75rem 1rem",
        fontSize: "0.875rem",
        fontWeight: 500,
      }}
    >
      {message}
      {mailto && (
        <>
          {" "}
          <a href={mailto} style={{ color: "inherit", textDecoration: "underline", fontWeight: 700 }}>
            {linkLabel}
          </a>
        </>
      )}
      {children}
    </div>
  );
}
