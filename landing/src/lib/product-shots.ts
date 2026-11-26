/** Finds a screenshot by its file name without extension. A missing one stops the build: a page never ships with an empty image slot. */
export function findShot<T>(shots: Record<string, T>, name: string): T {
  const entry = Object.entries(shots).find(([path]) => path.split("/").pop()?.replace(/\.[^.]+$/, "") === name);
  if (!entry) throw new Error(`Missing product image "${name}" in src/assets/screens/`);
  return entry[1];
}
