import { cellApplies, PASTE_COLUMNS, type Column, type GridRow } from "./state.ts";
import type { GridType } from "./values.ts";

export const PAGE_SIZE = 100;

export const GRID_COLUMNS: Column[] = [...PASTE_COLUMNS, "detail"];

function fold(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
}

export function filterRows(rows: readonly GridRow[], type: GridType | "", search: string): GridRow[] {
  const needle = fold(search.trim());
  return rows.filter((r) => (!type || r.type === type) && (!needle || fold(r.name).indexOf(needle) >= 0));
}

export function pageCount(total: number): number {
  return Math.max(1, Math.ceil(total / PAGE_SIZE));
}

export function pageRows<T>(rows: readonly T[], page: number): T[] {
  const safe = Math.min(Math.max(0, page), pageCount(rows.length) - 1);
  return rows.slice(safe * PAGE_SIZE, (safe + 1) * PAGE_SIZE);
}

export interface CellPosition {
  rowId: string;
  column: Column;
}

export type MoveKey = "ArrowUp" | "ArrowDown" | "ArrowLeft" | "ArrowRight" | "Enter" | "Tab" | "ShiftTab";

// Moves skip cells that do not apply to a row's type, so focus never lands on a disabled cell.
export function moveCell(rows: readonly GridRow[], from: CellPosition, key: MoveKey): CellPosition | null {
  const r = rows.findIndex((row) => row.id === from.rowId);
  if (r < 0) return null;
  const c = GRID_COLUMNS.indexOf(from.column);

  if (key === "ArrowUp" || key === "ArrowDown" || key === "Enter") {
    const step = key === "ArrowUp" ? -1 : 1;
    for (let i = r + step; i >= 0 && i < rows.length; i += step) {
      if (cellApplies(rows[i].type, from.column)) return { rowId: rows[i].id, column: from.column };
    }
    return null;
  }

  const step = key === "ArrowLeft" || key === "ShiftTab" ? -1 : 1;
  const wraps = key === "Tab" || key === "ShiftTab";
  let row = r;
  let col = c;
  for (;;) {
    col += step;
    if (col < 0 || col >= GRID_COLUMNS.length) {
      if (!wraps) return null;
      row += step;
      if (row < 0 || row >= rows.length) return null;
      col = step > 0 ? 0 : GRID_COLUMNS.length - 1;
    }
    if (cellApplies(rows[row].type, GRID_COLUMNS[col])) return { rowId: rows[row].id, column: GRID_COLUMNS[col] };
  }
}
