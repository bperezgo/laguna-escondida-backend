import Fuse from "fuse.js";
import type { IFuseOptions, FuseOptionKey } from "fuse.js";

/**
 * Shared fuzzy-search helper for in-memory lists (products, orders, …).
 *
 * Why this exists: every list search in the app was an accent-sensitive
 * `.toLowerCase().includes()`, so in a Spanish-language POS "camaron" would
 * miss "Camarón" and a single typo returned nothing. This centralises the
 * behaviour on top of Fuse.js so matching is accent-folded and typo-tolerant,
 * and every surface behaves the same way.
 */

export interface FuzzyOptions<T> {
  /** Fields to search, with optional per-field weights (higher = more relevant). */
  keys: FuseOptionKey<T>[];
  /**
   * Match tolerance: 0.0 = only exact substrings, 1.0 = match almost anything.
   * Defaults to 0.4 — loose enough for one or two typos, tight enough to avoid
   * noise on short menus.
   */
  threshold?: number;
}

/**
 * Fuzzy-filter `items` by `query`, returning matches in descending relevance.
 *
 * Accent-insensitive (café ≈ cafe) and typo-tolerant (cmaron ≈ camarón) via
 * Fuse.js. An empty/whitespace query returns the original list unchanged, so
 * callers can use the result directly as their display list.
 *
 * Builds a fresh index per call: fine for the small in-memory lists we search
 * (menus, open bills). If a list ever grows large, memoise the `Fuse` instance.
 */
export function fuzzyFilter<T>(
  items: T[],
  query: string,
  options: FuzzyOptions<T>,
): T[] {
  const q = query.trim();
  if (!q) return items;

  const fuseOptions: IFuseOptions<T> = {
    keys: options.keys,
    threshold: options.threshold ?? 0.4,
    // Fold accents on both the query and the indexed values (Fuse v7+).
    ignoreDiacritics: true,
    // Match anywhere in the field, not just near the start.
    ignoreLocation: true,
  };

  const fuse = new Fuse(items, fuseOptions);
  return fuse.search(q).map((result) => result.item);
}
