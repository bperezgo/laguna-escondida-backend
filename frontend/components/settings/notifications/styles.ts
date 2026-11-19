export const muted = { color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

export const row = { display: "flex", gap: "0.5rem", flexWrap: "wrap", alignItems: "center" } as const;

export const confirmBox = {
  padding: "1rem",
  border: "1px solid var(--color-danger)",
  borderRadius: "var(--radius-sm)",
  display: "flex",
  flexDirection: "column",
  gap: "0.75rem",
} as const;
