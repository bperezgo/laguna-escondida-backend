export type GridAccess = "loading" | "edge" | "forbidden" | "ready";

// Only a convenience: the import endpoints refuse callers without catalog:import themselves.
export function gridAccess(input: { isLoading: boolean; isEdge: boolean; canImport: boolean }): GridAccess {
  if (input.isLoading) return "loading";
  if (input.isEdge) return "edge";
  return input.canImport ? "ready" : "forbidden";
}

export function showGridLink(input: { isLoading: boolean; isEdge: boolean; canImport: boolean }): boolean {
  return gridAccess(input) === "ready";
}
