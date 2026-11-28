import { MAX_ROWS, PASTE_COLUMNS, type Column, type PasteCell } from "./state.ts";
import { parseAmount, parseArea, parseCount, parsePercent, parseType, parseUnit } from "./values.ts";

// Spreadsheets copy as tab-separated text, quoting a cell that holds a tab, a newline or a quote.
export function parseClipboard(text: string): string[][] {
  const lines: string[][] = [];
  let line: string[] = [];
  let cell = "";
  let quoted = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"' && text[i + 1] === '"') {
        cell += '"';
        i++;
      } else if (ch === '"') {
        quoted = false;
      } else {
        cell += ch;
      }
    } else if (ch === '"' && cell === "") {
      quoted = true;
    } else if (ch === "\t") {
      line.push(cell);
      cell = "";
    } else if (ch === "\n" || ch === "\r") {
      if (ch === "\r" && text[i + 1] === "\n") i++;
      line.push(cell);
      lines.push(line);
      line = [];
      cell = "";
    } else {
      cell += ch;
    }
  }
  if (cell !== "" || line.length > 0) {
    line.push(cell);
    lines.push(line);
  }
  return lines;
}

function numeric(raw: string, parse: (s: string) => number | null): PasteCell {
  if (raw === "") return { value: "" };
  const n = parse(raw);
  return n === null ? { invalid: true } : { value: String(n) };
}

export function pasteCell(column: Column, raw: string): PasteCell {
  const value = raw.trim();
  switch (column) {
    case "type": {
      if (value === "") return { skip: true };
      const type = parseType(value);
      return type ? { value: type } : { invalid: true };
    }
    case "unit": {
      if (value === "") return { skip: true };
      const unit = parseUnit(value);
      return unit ? { value: unit } : { invalid: true };
    }
    case "area":
      return { value: parseArea(value) };
    case "price":
      return numeric(value, parseAmount);
    case "ico":
    case "vat":
      return numeric(value, parsePercent);
    case "stock":
      return numeric(value, parseCount);
    default:
      return { value };
  }
}

// Cells past the last column are dropped, and at most MAX_ROWS lines are read: the reducer
// also stops adding rows once the grid holds MAX_ROWS.
export function pasteCells(text: string, startColumn: Column): PasteCell[][] {
  const first = PASTE_COLUMNS.indexOf(startColumn);
  if (first < 0) return [];
  return parseClipboard(text)
    .slice(0, MAX_ROWS)
    .map((cells) =>
      cells.slice(0, PASTE_COLUMNS.length - first).map((raw, offset) => pasteCell(PASTE_COLUMNS[first + offset], raw)),
    );
}
