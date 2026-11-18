export function ErrorBanner({ message }: { message: string }) {
  return (
    <div
      role="alert"
      style={{
        padding: "1rem",
        backgroundColor: "var(--color-danger-light)",
        color: "var(--color-danger)",
        border: "1px solid var(--color-danger)",
        borderRadius: "var(--radius-sm)",
      }}
    >
      <strong>Error:</strong> {message}
    </div>
  );
}

export function NoticeBanner({ message }: { message: string }) {
  return (
    <div
      role="status"
      style={{
        padding: "0.75rem 1rem",
        backgroundColor: "var(--color-warning-bg)",
        color: "var(--color-warning-text)",
        borderRadius: "var(--radius-sm)",
      }}
    >
      {message}
    </div>
  );
}
