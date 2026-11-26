import type { ReactNode } from "react";

export default function PublicShell({
  subtitle,
  maxWidth = "26rem",
  children,
}: {
  subtitle: ReactNode;
  maxWidth?: string;
  children: ReactNode;
}) {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: "var(--color-bg)",
        padding: "1rem",
      }}
    >
      <div style={{ maxWidth, width: "100%" }}>
        <div style={{ textAlign: "center", marginBottom: "1.75rem" }}>
          <img src="/logo-navbar.png" alt="Kuenta" style={{ height: "72px", marginBottom: "1rem" }} />
          <p style={{ fontSize: "0.95rem", color: "var(--color-text-secondary)", margin: 0 }}>{subtitle}</p>
        </div>
        {children}
      </div>
    </div>
  );
}
