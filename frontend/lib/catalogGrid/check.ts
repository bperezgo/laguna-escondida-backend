import type {
  CatalogImportDocument,
  CatalogImportProblem,
  CatalogImportResult,
} from "../../types/catalogImport.ts";
import type { Column } from "./state.ts";
import { locateProblem, type PanelTarget, type ProblemTarget } from "./toDocument.ts";

export const VALIDATE_URL = "/api/catalog/import/validate";
export const APPLY_URL = "/api/catalog/import";

export type FetchLike = (url: string, init: RequestInit) => Promise<Response>;

export type ImportOutcome =
  | { kind: "result"; status: number; result: CatalogImportResult }
  | { kind: "error"; status: number; message: string };

// Not a security digest: it only tells the page whether the grid changed since the check, and
// the server compares its own digest on create anyway.
export function documentHash(document: CatalogImportDocument): string {
  const text = JSON.stringify(document);
  let h = 0x811c9dc5;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return `${text.length}:${(h >>> 0).toString(16)}`;
}

function errorMessage(status: number, body: unknown): string {
  switch (status) {
    case 401:
      return "Tu sesión terminó. Vuelve a iniciar sesión.";
    case 403:
      return "No tienes permiso para importar el catálogo.";
    case 404:
      return "La importación del catálogo solo está disponible en la nube.";
    case 413:
      return "El catálogo es demasiado grande para enviarlo (máximo 2 MiB).";
  }
  const error = typeof body === "object" && body !== null ? (body as { error?: unknown }).error : undefined;
  return typeof error === "string" && error ? error : "No se pudo contactar al servidor. Intenta de nuevo.";
}

function isResult(body: unknown): body is CatalogImportResult {
  return typeof body === "object" && body !== null && Array.isArray((body as CatalogImportResult).problems);
}

export async function postImport(fetchFn: FetchLike, url: string, body: unknown): Promise<ImportOutcome> {
  let response: Response;
  try {
    response = await fetchFn(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    return { kind: "error", status: 0, message: errorMessage(0, null) };
  }
  const parsed: unknown = await response.json().catch(() => null);
  // 200 and 201 answer a check or a create; 409 and 422 refuse a create with problems.
  if ([200, 201, 409, 422].indexOf(response.status) >= 0 && isResult(parsed)) {
    return { kind: "result", status: response.status, result: parsed };
  }
  return { kind: "error", status: response.status, message: errorMessage(response.status, parsed) };
}

export interface PassedCheck {
  digest: string;
  documentHash: string;
}

export function passedCheck(outcome: ImportOutcome, document: CatalogImportDocument): PassedCheck | null {
  if (outcome.kind !== "result" || !outcome.result.valid || outcome.result.problems.length > 0) return null;
  return { digest: outcome.result.digest, documentHash: documentHash(document) };
}

export function canCreate(check: PassedCheck | null, document: CatalogImportDocument): boolean {
  return check !== null && check.documentHash === documentHash(document);
}

export async function checkCatalog(fetchFn: FetchLike, document: CatalogImportDocument): Promise<ImportOutcome> {
  return postImport(fetchFn, VALIDATE_URL, document);
}

export type CreateOutcome =
  | { kind: "created"; products: number }
  | { kind: "refused"; outcome: ImportOutcome };

// The draft is cleared only once the server says the catalog was applied.
export async function createCatalog(
  fetchFn: FetchLike,
  document: CatalogImportDocument,
  check: PassedCheck,
  onCreated: () => void,
): Promise<CreateOutcome> {
  const outcome = await postImport(fetchFn, APPLY_URL, { ...document, expected_digest: check.digest });
  if (outcome.kind === "result" && outcome.result.applied) {
    onCreated();
    return { kind: "created", products: outcome.result.summary.products };
  }
  return { kind: "refused", outcome };
}

export interface PanelProblem {
  panel: PanelTarget;
  message: string;
}

export interface PlacedProblems {
  cells: Record<string, Partial<Record<Column, string[]>>>;
  panels: Record<string, PanelProblem[]>;
  general: CatalogImportProblem[];
}

export const NO_PROBLEMS: PlacedProblems = { cells: {}, panels: {}, general: [] };

export function placeProblems(
  problems: readonly CatalogImportProblem[],
  pointers: Record<string, ProblemTarget>,
): PlacedProblems {
  const placed: PlacedProblems = { cells: {}, panels: {}, general: [] };
  for (const problem of problems) {
    const target = locateProblem(pointers, problem);
    if (!target) {
      placed.general.push(problem);
      continue;
    }
    const cell = target.cell ?? "name";
    const row = (placed.cells[target.rowId] ??= {});
    (row[cell] ??= []).push(problem.message);
    if (target.panel) (placed.panels[target.rowId] ??= []).push({ panel: target.panel, message: problem.message });
  }
  return placed;
}
