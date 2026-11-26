"use client";

import { useState } from "react";
import { Button } from "@/components/ui";

export default function CopyField({ id, label, value }: { id: string; label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}>
      <label htmlFor={id} style={{ fontSize: "0.85rem", color: "var(--color-text-secondary)" }}>
        {label}
      </label>
      <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
        <input
          id={id}
          readOnly
          value={value}
          spellCheck={false}
          autoComplete="off"
          onFocus={(e) => e.currentTarget.select()}
          style={{
            flex: 1,
            minWidth: 0,
            fontFamily: "monospace",
            fontSize: "0.95rem",
            padding: "0.5rem 0.75rem",
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--color-border)",
            backgroundColor: "var(--color-surface)",
            color: "var(--color-text-primary)",
          }}
        />
        <Button size="sm" variant="secondary" type="button" onClick={copy}>
          {copied ? "Copiado" : "Copiar"}
        </Button>
      </div>
    </div>
  );
}
