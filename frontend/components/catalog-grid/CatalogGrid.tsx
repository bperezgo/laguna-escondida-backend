"use client";

import { useEffect, useMemo, useReducer, useRef, useState, type ChangeEvent, type ClipboardEvent, type KeyboardEvent } from "react";
import Link from "next/link";
import { Button, Input, Select } from "@/components/ui";
import {
  EMPTY_STATE,
  gridReducer,
  gridSkus,
  ingredientCandidates,
  newRow,
  pasteImpact,
  platesUsing,
  typeChangeImpact,
  type Column,
  type ExistingProduct,
  type GridRow,
  type GridState,
  type TextColumn,
} from "@/lib/catalogGrid/state";
import { toDocument } from "@/lib/catalogGrid/toDocument";
import {
  canCreate,
  checkCatalog,
  createCatalog,
  NO_PROBLEMS,
  passedCheck,
  placeProblems,
  type ImportOutcome,
  type PassedCheck,
  type PlacedProblems,
} from "@/lib/catalogGrid/check";
import { readDocumentFile } from "@/lib/catalogGrid/fromDocument";
import { pasteCells } from "@/lib/catalogGrid/paste";
import { browserStorage, clearDraft, loadDraft, saveDraft } from "@/lib/catalogGrid/draft";
import { filterRows, moveCell, pageCount, pageRows, PAGE_SIZE, type MoveKey } from "@/lib/catalogGrid/view";
import { areaLabel, GRID_AREAS, GRID_TYPES, GRID_UNITS, parseArea, type GridType } from "@/lib/catalogGrid/values";
import GridRowView from "@/components/catalog-grid/GridRowView";
import PlatePanel from "@/components/catalog-grid/PlatePanel";
import TaxColumnMenu from "@/components/catalog-grid/TaxColumnMenu";

interface CatalogGridProps {
  organizationId: string;
  userId: string;
  existing: ExistingProduct[];
}

const SAVE_DELAY_MS = 1000;

const banner = (tone: "danger" | "info" | "success") =>
  ({
    padding: "0.75rem 1rem",
    borderRadius: "var(--radius-sm)",
    border: `1px solid var(--color-${tone === "info" ? "primary" : tone})`,
    backgroundColor: `var(--color-${tone === "info" ? "primary" : tone}-light)`,
    color: "var(--color-text-primary)",
    fontSize: "0.875rem",
  }) as const;

const th = {
  padding: "0.5rem",
  textAlign: "left",
  fontSize: "0.75rem",
  fontWeight: 600,
  color: "var(--color-text-secondary)",
  borderBottom: "1px solid var(--color-border)",
  whiteSpace: "nowrap",
} as const;

function freshState(): GridState {
  return { rows: [newRow()], unmatched: [] };
}

function downloadJson(text: string) {
  const blob = new Blob([text], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `catalogo-${new Date().toISOString().slice(0, 10)}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

export default function CatalogGrid({ organizationId, userId, existing }: CatalogGridProps) {
  // CatalogGrid mounts only in the browser, after the products load, so reading storage here
  // never differs from a server render.
  const [initial] = useState(() => loadDraft(browserStorage(), organizationId, userId));
  const [state, dispatch] = useReducer(gridReducer, initial, (loaded) =>
    loaded.state && loaded.state.rows.length > 0 ? loaded.state : freshState(),
  );
  const [storageOk, setStorageOk] = useState(initial.available);
  const saving = useRef(true);

  const [typeFilter, setTypeFilter] = useState<GridType | "">("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<string[]>([]);
  const [addType, setAddType] = useState<GridType>("SELLABLE");
  const [openPlate, setOpenPlate] = useState<string | null>(null);

  const [check, setCheck] = useState<PassedCheck | null>(null);
  const [problems, setProblems] = useState<PlacedProblems>(NO_PROBLEMS);
  const [status, setStatus] = useState<{ tone: "danger" | "info" | "success"; text: string } | null>(null);
  const [busy, setBusy] = useState<"check" | "create" | "load" | null>(null);
  const [created, setCreated] = useState<number | null>(null);

  const tableRef = useRef<HTMLDivElement>(null);
  const focused = useRef<{ rowId: string; column: Column } | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!saving.current) return;
    const timer = setTimeout(() => setStorageOk(saveDraft(browserStorage(), organizationId, userId, state)), SAVE_DELAY_MS);
    return () => clearTimeout(timer);
  }, [state, organizationId, userId]);

  const organizationSkus = useMemo(() => existing.map((p) => p.sku), [existing]);
  const skus = useMemo(() => gridSkus(state.rows, organizationSkus), [state.rows, organizationSkus]);
  const built = useMemo(() => toDocument(state, existing, organizationSkus), [state, existing, organizationSkus]);
  const createEnabled = canCreate(check, built.document) && busy === null;

  const filtered = useMemo(() => filterRows(state.rows, typeFilter, search), [state.rows, typeFilter, search]);
  const pages = pageCount(filtered.length);
  const currentPage = Math.min(page, pages - 1);
  const visible = pageRows(filtered, currentPage);
  const indexById = useMemo(() => {
    const m: Record<string, number> = {};
    state.rows.forEach((r, i) => (m[r.id] = i + 1));
    return m;
  }, [state.rows]);

  const plateRow = openPlate ? state.rows.find((r) => r.id === openPlate && r.type === "COMPOSITE") : undefined;
  const candidates = useMemo(
    () => (plateRow ? ingredientCandidates(state, plateRow.id, existing) : []),
    [plateRow, state, existing],
  );

  const showOutcome = (outcome: ImportOutcome, after: "check" | "create") => {
    if (outcome.kind === "error") {
      setStatus({ tone: "danger", text: outcome.message });
      return;
    }
    const placed = placeProblems(outcome.result.problems, built.pointers);
    setProblems(placed);
    const count = outcome.result.problems.length;
    if (count === 0 && after === "check") {
      setStatus({ tone: "success", text: `Revisión sin problemas: se crearán ${outcome.result.summary.products} productos.` });
    } else {
      setStatus({ tone: "danger", text: `Se encontraron ${count} ${count === 1 ? "problema" : "problemas"}. Corrígelos y revisa de nuevo.` });
    }
  };

  const handleCheck = async () => {
    setBusy("check");
    setStatus(null);
    const document = built.document;
    const outcome = await checkCatalog(fetch, document);
    setCheck(passedCheck(outcome, document));
    showOutcome(outcome, "check");
    setBusy(null);
  };

  const handleCreate = async () => {
    if (!check) return;
    setBusy("create");
    setStatus(null);
    const got = await createCatalog(fetch, built.document, check, () => clearDraft(browserStorage(), organizationId, userId));
    if (got.kind === "created") {
      setCreated(got.products);
      setCheck(null);
      setProblems(NO_PROBLEMS);
      saving.current = false;
      dispatch({ type: "replace", state: EMPTY_STATE });
    } else {
      setCheck(null);
      showOutcome(got.outcome, "create");
    }
    setBusy(null);
  };

  const changeType = (row: GridRow, next: GridType) => {
    const impact = typeChangeImpact(state, row.id, next);
    if (impact) {
      const parts: string[] = [];
      if (impact.plates.length > 0) parts.push(`Se quitará de: ${impact.plates.join(", ")}.`);
      if (impact.dropsPlateDetail) parts.push("Se borrarán su receta y sus opciones.");
      if (!window.confirm(`¿Cambiar el tipo de "${row.name.trim() || "esta fila"}"? ${parts.join(" ")}`)) return;
    }
    dispatch({ type: "changeType", id: row.id, productType: next });
  };

  const deleteRows = (ids: string[]) => {
    if (ids.length === 0) return;
    const plates = platesUsing(state, ids);
    if (plates.length > 0 && !window.confirm(`Estas filas se usan en: ${plates.join(", ")}. Se quitarán de esas recetas y opciones. ¿Eliminar?`)) {
      return;
    }
    dispatch({ type: "delete", ids });
    setSelected((prev) => prev.filter((id) => ids.indexOf(id) < 0));
  };

  const edit = (id: string, column: TextColumn, value: string) => dispatch({ type: "edit", id, column, value });

  const focusCell = (rowId: string, column: Column) => {
    const el = tableRef.current?.querySelector<HTMLElement>(`[data-cell="${rowId}:${column}"]`);
    el?.focus();
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const target = e.target as HTMLElement;
    const cell = target.getAttribute("data-cell");
    if (!cell) return;
    const [rowId, column] = cell.split(":") as [string, Column];
    let key: MoveKey | null = null;
    if (e.key === "ArrowUp" || e.key === "ArrowDown") key = e.key;
    else if (e.key === "Enter" && target.tagName === "INPUT") key = "Enter";
    else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      const input = target as HTMLInputElement;
      const atEdge =
        target.tagName !== "INPUT" ||
        (e.key === "ArrowLeft" ? input.selectionStart === 0 : input.selectionEnd === input.value.length);
      if (atEdge) key = e.key;
    }
    if (!key) return;
    if (target.tagName === "SELECT" && (e.key === "ArrowUp" || e.key === "ArrowDown")) return;
    const next = moveCell(visible, { rowId, column }, key);
    if (!next) return;
    e.preventDefault();
    focusCell(next.rowId, next.column);
  };

  const handlePaste = (e: ClipboardEvent<HTMLDivElement>) => {
    const text = e.clipboardData.getData("text/plain");
    if (!/[\t\n]/.test(text.replace(/\r?\n$/, ""))) return;
    const at = focused.current;
    if (!at || at.column === "detail") return;
    e.preventDefault();
    const before = state.rows.length;
    const cells = pasteCells(text, at.column);
    const order = filtered.map((r) => r.id);
    const action = { type: "paste" as const, startId: at.rowId, startColumn: at.column, order, cells };
    const plates = pasteImpact(state, action);
    if (
      plates.length > 0 &&
      !window.confirm(`Lo pegado cambia el tipo de ingredientes que usan: ${plates.join(", ")}. Se quitarán de esas recetas y opciones. ¿Pegar?`)
    ) {
      return;
    }
    dispatch(action);
    const room = 1000 - before;
    const needed = Math.max(0, order.indexOf(at.rowId) + cells.length - order.length);
    if (needed > room) setStatus({ tone: "info", text: "El catálogo admite hasta 1.000 filas; el resto de lo pegado no se agregó." });
  };

  const handleFile = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    const hasRows = state.rows.some((r) => r.name.trim() !== "");
    if (hasRows && !window.confirm("Cargar el archivo reemplaza las filas actuales. ¿Continuar?")) return;
    setBusy("load");
    const loaded = await readDocumentFile(file, existing);
    setBusy(null);
    if (!loaded.ok) {
      setStatus({ tone: "danger", text: loaded.message });
      return;
    }
    dispatch({ type: "replace", state: loaded.state });
    setCheck(null);
    setProblems(NO_PROBLEMS);
    setSelected([]);
    setPage(0);
    const missing = loaded.state.unmatched.length;
    setStatus({
      tone: "info",
      text:
        `Se cargaron ${loaded.state.rows.length} filas. Nada se crea hasta que revises y crees el catálogo.` +
        (missing > 0 ? ` ${missing} productos existentes no se encontraron y están marcados.` : ""),
    });
  };

  const discard = () => {
    if (!window.confirm("¿Descartar el borrador? Se borran todas las filas.")) return;
    clearDraft(browserStorage(), organizationId, userId);
    dispatch({ type: "replace", state: freshState() });
    setCheck(null);
    setProblems(NO_PROBLEMS);
    setSelected([]);
    setStatus(null);
  };

  if (created !== null) {
    return (
      <div style={{ maxWidth: "720px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
        <h1 style={{ margin: 0, color: "var(--color-text-primary)" }}>Catálogo creado</h1>
        <div style={banner("success")}>Se crearon {created} productos.</div>
        <div style={{ display: "flex", gap: "0.75rem" }}>
          <Link href="/products" style={{ color: "var(--color-primary)", fontWeight: 600 }}>
            Ver productos
          </Link>
          <button
            type="button"
            onClick={() => {
              setCreated(null);
              saving.current = true;
              dispatch({ type: "replace", state: freshState() });
            }}
            style={{ background: "none", border: "none", color: "var(--color-primary)", cursor: "pointer" }}
          >
            Empezar otro catálogo
          </button>
        </div>
      </div>
    );
  }

  const pageIds = visible.map((r) => r.id);
  const allPageSelected = pageIds.length > 0 && pageIds.every((id) => selected.indexOf(id) >= 0);

  return (
    <div style={{ maxWidth: "1600px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
      <div>
        <h1 style={{ margin: 0, fontSize: "2rem", color: "var(--color-text-primary)" }}>Catálogo inicial</h1>
        <p style={{ margin: "0.5rem 0 0", color: "var(--color-text-secondary)" }}>
          Una fila por producto nuevo. Pega desde una hoja de cálculo (nombre y precio, por ejemplo), revisa y crea todo de una vez.{" "}
          {storageOk
            ? "El borrador se guarda en este navegador."
            : "Este navegador no permite guardar: el borrador no se está guardando. Exporta el JSON para no perderlo."}
        </p>
      </div>

      <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem", alignItems: "center" }}>
        <Select
          value={addType}
          onChange={(e) => setAddType(e.target.value as GridType)}
          aria-label="Tipo de la fila nueva"
          style={{ minWidth: "10rem" }}
        >
          {GRID_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </Select>
        <Button variant="secondary" onClick={() => dispatch({ type: "add", productType: addType })} disabled={state.rows.length >= 1000}>
          + Agregar fila
        </Button>
        <Button variant="secondary" onClick={() => deleteRows(selected)} disabled={selected.length === 0}>
          Eliminar seleccionadas ({selected.length})
        </Button>
        <div style={{ flex: 1 }} />
        <Button variant="ghost" onClick={() => downloadJson(JSON.stringify(built.document, null, 2))}>
          Exportar JSON
        </Button>
        <Button variant="ghost" onClick={() => fileInput.current?.click()} disabled={busy !== null}>
          Cargar JSON
        </Button>
        <input ref={fileInput} type="file" accept="application/json,.json" onChange={handleFile} style={{ display: "none" }} />
        <Button variant="ghost" onClick={discard}>
          Descartar borrador
        </Button>
        <Button variant="secondary" onClick={handleCheck} disabled={busy !== null || state.rows.length === 0}>
          {busy === "check" ? "Revisando…" : "Revisar"}
        </Button>
        <Button onClick={handleCreate} disabled={!createEnabled}>
          {busy === "create" ? "Creando…" : "Crear catálogo"}
        </Button>
      </div>

      {status && (
        <div role="status" style={banner(status.tone)}>
          {status.text}
        </div>
      )}
      {check && !createEnabled && busy === null && (
        <div style={banner("info")}>El catálogo cambió desde la última revisión. Revisa de nuevo para poder crearlo.</div>
      )}
      {problems.general.length > 0 && (
        <div role="alert" style={banner("danger")}>
          <strong>Problemas del catálogo</strong>
          <ul style={{ margin: "0.25rem 0 0", paddingLeft: "1.25rem" }}>
            {problems.general.map((p, i) => (
              <li key={i}>{p.message}</li>
            ))}
          </ul>
        </div>
      )}

      <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem", alignItems: "center" }}>
        <Select
          value={typeFilter}
          onChange={(e) => {
            setTypeFilter(e.target.value as GridType | "");
            setPage(0);
          }}
          aria-label="Filtrar por tipo"
          style={{ minWidth: "11rem" }}
        >
          <option value="">Todos los tipos</option>
          {GRID_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </Select>
        <Input
          type="search"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          placeholder="Buscar por nombre"
          aria-label="Buscar por nombre"
          style={{ minWidth: "16rem" }}
        />
        <span style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
          {filtered.length} de {state.rows.length} filas
        </span>
      </div>

      <datalist id="catalog-grid-areas">
        {GRID_AREAS.map((a) => (
          <option key={a.value} value={a.label} />
        ))}
      </datalist>

      <div
        ref={tableRef}
        onKeyDown={handleKeyDown}
        onPaste={handlePaste}
        onFocus={(e) => {
          const cell = (e.target as HTMLElement).getAttribute("data-cell");
          if (cell) {
            const [rowId, column] = cell.split(":") as [string, Column];
            focused.current = { rowId, column };
          }
        }}
        style={{ overflowX: "auto", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", backgroundColor: "var(--color-surface)" }}
      >
        <table style={{ borderCollapse: "collapse", width: "100%" }}>
          <thead>
            <tr>
              <th style={th}>
                <input
                  type="checkbox"
                  aria-label="Seleccionar las filas de esta página"
                  checked={allPageSelected}
                  onChange={() =>
                    setSelected((prev) =>
                      allPageSelected ? prev.filter((id) => pageIds.indexOf(id) < 0) : prev.concat(pageIds.filter((id) => prev.indexOf(id) < 0)),
                    )
                  }
                />
              </th>
              <th style={th}>#</th>
              <th style={th}>Nombre</th>
              <th style={th}>Precio</th>
              <th style={th}>Tipo</th>
              <th style={th}>Categoría</th>
              <th style={th}>Área</th>
              <th style={th}>Unidad</th>
              <th style={th}>
                <TaxColumnMenu
                  label="ICO"
                  selectedCount={selected.length}
                  onFillAll={(value) => dispatch({ type: "fillTax", column: "ico", value })}
                  onFillSelected={(value) => dispatch({ type: "fillTax", column: "ico", value, ids: selected })}
                  onClear={() => dispatch({ type: "clearTax", column: "ico" })}
                />
              </th>
              <th style={th}>
                <TaxColumnMenu
                  label="IVA"
                  selectedCount={selected.length}
                  onFillAll={(value) => dispatch({ type: "fillTax", column: "vat", value })}
                  onFillSelected={(value) => dispatch({ type: "fillTax", column: "vat", value, ids: selected })}
                  onClear={() => dispatch({ type: "clearTax", column: "vat" })}
                />
              </th>
              <th style={th}>SKU</th>
              <th style={th}>Stock inicial</th>
              <th style={th}>Receta</th>
              <th style={th} />
            </tr>
          </thead>
          <tbody>
            {visible.map((row) => (
              <GridRowView
                key={row.id}
                row={row}
                index={indexById[row.id]}
                sku={skus[row.id]}
                problems={problems.cells[row.id]}
                selected={selected.indexOf(row.id) >= 0}
                areaText={areaLabel(row.area)}
                units={GRID_UNITS}
                onSelect={(on) => setSelected((prev) => (on ? prev.concat([row.id]) : prev.filter((id) => id !== row.id)))}
                onEdit={(column, value) => edit(row.id, column, column === "area" ? parseArea(value) : value)}
                onType={(next) => changeType(row, next)}
                onOpenPlate={() => setOpenPlate(row.id)}
                onDuplicate={() => dispatch({ type: "duplicate", id: row.id })}
                onDelete={() => deleteRows([row.id])}
              />
            ))}
          </tbody>
        </table>
      </div>

      {pages > 1 && (
        <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
          <Button variant="ghost" size="sm" onClick={() => setPage(currentPage - 1)} disabled={currentPage === 0}>
            ← Anterior
          </Button>
          <span style={{ fontSize: "0.875rem", color: "var(--color-text-secondary)" }}>
            Filas {currentPage * PAGE_SIZE + 1}–{Math.min(filtered.length, (currentPage + 1) * PAGE_SIZE)} de {filtered.length}
          </span>
          <Button variant="ghost" size="sm" onClick={() => setPage(currentPage + 1)} disabled={currentPage >= pages - 1}>
            Siguiente →
          </Button>
        </div>
      )}

      {plateRow && (
        <PlatePanel
          key={plateRow.id}
          plate={plateRow}
          candidates={candidates}
          serverProblems={problems.panels[plateRow.id] ?? []}
          onApply={(recipe, slots) => {
            dispatch({ type: "setPlate", id: plateRow.id, recipe, slots });
            setOpenPlate(null);
          }}
          onCancel={() => setOpenPlate(null)}
        />
      )}
    </div>
  );
}
