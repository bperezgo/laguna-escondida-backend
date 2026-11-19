import type { ReactNode } from "react";

interface SettingsSectionShellProps {
  title: string;
  children: ReactNode;
}

export default function SettingsSectionShell({ title, children }: SettingsSectionShellProps) {
  return (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "2rem" }}>
      <div style={{ maxWidth: "960px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
        <a href="/ajustes" style={{ color: "var(--color-text-secondary)", fontSize: "0.9rem", textDecoration: "none" }}>
          ← Ajustes
        </a>
        <h1 style={{ margin: 0, fontSize: "2rem", fontWeight: "bold", color: "var(--color-text-primary)" }}>{title}</h1>
        {children}
      </div>
    </div>
  );
}
