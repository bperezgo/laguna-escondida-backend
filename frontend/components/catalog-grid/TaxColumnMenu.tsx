"use client";

import { useState } from "react";
import { Button, Input } from "@/components/ui";
import { parsePercent } from "@/lib/catalogGrid/values";

interface TaxColumnMenuProps {
  label: string;
  selectedCount: number;
  onFillAll: (value: string) => void;
  onFillSelected: (value: string) => void;
  onClear: () => void;
}

const menuButton = {
  background: "none",
  border: "none",
  cursor: "pointer",
  color: "var(--color-primary)",
  padding: "0 0.25rem",
  fontSize: "0.8rem",
} as const;

export default function TaxColumnMenu({ label, selectedCount, onFillAll, onFillSelected, onClear }: TaxColumnMenuProps) {
  const [open, setOpen] = useState(false);
  const [value, setValue] = useState("");
  const percent = parsePercent(value);

  const run = (fn: () => void) => {
    fn();
    setOpen(false);
  };

  return (
    <span style={{ position: "relative", whiteSpace: "nowrap" }}>
      {label}
      <button type="button" style={menuButton} aria-label={`Acciones de ${label}`} aria-expanded={open} onClick={() => setOpen((o) => !o)}>
        ▾
      </button>
      {open && (
        <div
          role="menu"
          style={{
            position: "absolute",
            top: "1.5rem",
            left: 0,
            zIndex: 20,
            display: "flex",
            flexDirection: "column",
            gap: "0.5rem",
            padding: "0.75rem",
            minWidth: "14rem",
            backgroundColor: "var(--color-surface)",
            border: "1px solid var(--color-border)",
            borderRadius: "var(--radius-sm)",
            boxShadow: "var(--shadow-sm)",
            fontWeight: 400,
          }}
        >
          <Input
            label="Valor (%)"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            inputMode="decimal"
            placeholder="19"
            autoFocus
          />
          <Button
            variant="secondary"
            size="sm"
            role="menuitem"
            disabled={percent === null}
            onClick={() => run(() => onFillAll(String(percent)))}
          >
            Llenar todas las filas
          </Button>
          <Button
            variant="secondary"
            size="sm"
            role="menuitem"
            disabled={percent === null || selectedCount === 0}
            onClick={() => run(() => onFillSelected(String(percent)))}
          >
            Llenar seleccionadas ({selectedCount})
          </Button>
          <Button variant="ghost" size="sm" role="menuitem" onClick={() => run(onClear)}>
            Dejar la columna en 0 %
          </Button>
          <span style={{ fontSize: "0.7rem", color: "var(--color-text-secondary)" }}>Solo cambia filas que se venden.</span>
        </div>
      )}
    </span>
  );
}
