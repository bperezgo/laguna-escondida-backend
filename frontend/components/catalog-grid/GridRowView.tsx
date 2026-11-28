"use client";

import { memo, type CSSProperties, type ReactNode } from "react";
import { cellApplies, plateSummary, type Column, type GridRow, type SkuView, type TextColumn } from "@/lib/catalogGrid/state";
import { GRID_TYPES, type GridType } from "@/lib/catalogGrid/values";
import type { UnitOfMeasure } from "@/types/product";

interface GridRowViewProps {
  row: GridRow;
  index: number;
  sku: SkuView;
  problems?: Partial<Record<Column, string[]>>;
  selected: boolean;
  areaText: string;
  units: UnitOfMeasure[];
  onSelect: (on: boolean) => void;
  onEdit: (column: TextColumn, value: string) => void;
  onType: (type: GridType) => void;
  onOpenPlate: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
}

const td: CSSProperties = {
  padding: "0.25rem",
  borderBottom: "1px solid var(--color-border)",
  verticalAlign: "top",
};

const control: CSSProperties = {
  width: "100%",
  minWidth: "5rem",
  padding: "0.3rem 0.4rem",
  border: "1px solid var(--color-border)",
  borderRadius: "var(--radius-sm)",
  backgroundColor: "var(--color-bg)",
  color: "var(--color-text-primary)",
  fontSize: "0.85rem",
};

const invalid: CSSProperties = { borderColor: "var(--color-danger, #e02424)" };

const message: CSSProperties = {
  marginTop: "0.15rem",
  color: "var(--color-danger, #e02424)",
  fontSize: "0.7rem",
  lineHeight: 1.3,
  maxWidth: "16rem",
};

const notApplicable: CSSProperties = { ...td, color: "var(--color-text-secondary)", textAlign: "center" };

function GridRowView({
  row,
  index,
  sku,
  problems,
  selected,
  areaText,
  units,
  onSelect,
  onEdit,
  onType,
  onOpenPlate,
  onDuplicate,
  onDelete,
}: GridRowViewProps) {
  const messagesFor = (column: Column): string[] => {
    const list = problems?.[column] ?? [];
    return row.marks[column] ? ["Valor no válido"].concat(list) : list;
  };

  const cell = (column: Column, render: (style: CSSProperties, bad: boolean) => ReactNode, width?: string) => {
    if (!cellApplies(row.type, column)) {
      return (
        <td key={column} style={notApplicable} aria-disabled="true">
          —
        </td>
      );
    }
    const messages = messagesFor(column);
    const bad = messages.length > 0;
    const style = { ...control, ...(width ? { minWidth: width } : {}), ...(bad ? invalid : {}) };
    return (
      <td key={column} style={td}>
        {render(style, bad)}
        {messages.map((m, i) => (
          <div key={i} style={message}>
            {m}
          </div>
        ))}
      </td>
    );
  };

  const text = (column: TextColumn, value: string, opts: { width?: string; placeholder?: string; inputMode?: "decimal" | "numeric"; list?: string; italic?: boolean } = {}) =>
    cell(
      column,
      (style, bad) => (
        <input
          data-cell={`${row.id}:${column}`}
          value={value}
          onChange={(e) => onEdit(column, e.target.value)}
          placeholder={opts.placeholder}
          inputMode={opts.inputMode}
          list={opts.list}
          aria-invalid={bad || undefined}
          aria-label={`${column} fila ${index}`}
          style={opts.italic ? { ...style, fontStyle: "italic" } : style}
        />
      ),
      opts.width,
    );

  return (
    <tr style={selected ? { backgroundColor: "var(--color-primary-light)" } : undefined}>
      <td style={td}>
        <input type="checkbox" checked={selected} onChange={(e) => onSelect(e.target.checked)} aria-label={`Seleccionar fila ${index}`} />
      </td>
      <td style={{ ...td, color: "var(--color-text-secondary)", fontSize: "0.75rem", paddingTop: "0.6rem" }}>{index}</td>
      {text("name", row.name, { width: "12rem", placeholder: "Nombre" })}
      {text("price", row.price, { width: "6rem", inputMode: "decimal", placeholder: "$" })}
      {cell("type", (style, bad) => (
        <select
          data-cell={`${row.id}:type`}
          value={row.type}
          onChange={(e) => onType(e.target.value as GridType)}
          aria-invalid={bad || undefined}
          aria-label={`Tipo fila ${index}`}
          style={style}
        >
          {GRID_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      ))}
      {text("category", row.category, { width: "8rem", placeholder: "Categoría" })}
      {text("area", areaText, { width: "7rem", list: "catalog-grid-areas", placeholder: "Cocina" })}
      {cell("unit", (style, bad) => (
        <select
          data-cell={`${row.id}:unit`}
          value={row.unit}
          onChange={(e) => onEdit("unit", e.target.value)}
          aria-invalid={bad || undefined}
          aria-label={`Unidad fila ${index}`}
          style={style}
        >
          {row.unit === "" && <option value="">—</option>}
          {units.map((u) => (
            <option key={u} value={u}>
              {u === "unit" ? "und" : u}
            </option>
          ))}
        </select>
      ))}
      {text("ico", row.ico, { width: "3.5rem", inputMode: "decimal" })}
      {text("vat", row.vat, { width: "3.5rem", inputMode: "decimal" })}
      {/* A suggested SKU is shown as the placeholder, so typing starts a new value and emptying the cell brings the suggestion back. */}
      {text("sku", sku.suggested ? "" : sku.value, { width: "9rem", placeholder: sku.suggested ? sku.value : "", italic: sku.suggested })}
      {text("stock", row.stock, { width: "5rem", inputMode: "numeric" })}
      {cell("detail", (style) => (
        <button
          type="button"
          data-cell={`${row.id}:detail`}
          onClick={onOpenPlate}
          style={{ ...style, cursor: "pointer", textAlign: "left", whiteSpace: "nowrap" }}
        >
          {plateSummary(row)}
        </button>
      ))}
      <td style={{ ...td, whiteSpace: "nowrap" }}>
        <button type="button" onClick={onDuplicate} aria-label={`Duplicar fila ${index}`} title="Duplicar" style={{ background: "none", border: "none", cursor: "pointer" }}>
          ⧉
        </button>
        <button type="button" onClick={onDelete} aria-label={`Eliminar fila ${index}`} title="Eliminar" style={{ background: "none", border: "none", cursor: "pointer" }}>
          ✕
        </button>
      </td>
    </tr>
  );
}

export default memo(GridRowView);
