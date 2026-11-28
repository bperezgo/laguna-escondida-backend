"use client";

import { useState, useEffect, useCallback } from "react";
import type { Product, ProductType, ProductIngredientWithProduct } from "@/types/product";
import { recipeApi } from "@/lib/api/recipe";
import { productsApi } from "@/lib/api/products";
import { stockApi } from "@/lib/api/stock";
import {
  recipeChange,
  type IngredientCandidate,
  type RecipeLineDraft,
} from "@/lib/products/recipeLines";
import RecipeLinesFields from "@/components/products/RecipeLinesFields";

interface IngredientsEditorProps {
  // The persisted composite product whose recipe we are editing.
  product: Product;
}

// Products that make sense as recipe ingredients: things that hold stock (INGREDIENT, BOTH)
// or expand into stock via their own recipe (COMPOSITE, multi-level). Pure SELLABLE excluded.
const INGREDIENT_CANDIDATE_TYPES: ProductType[] = [
  "INGREDIENT",
  "BOTH",
  "COMPOSITE",
];

function toCandidate(p: Product): IngredientCandidate {
  return { id: p.id, name: p.name, unit: p.unit_of_measure };
}

function toLine(ing: ProductIngredientWithProduct): RecipeLineDraft {
  return {
    key: ing.id,
    ingredientId: ing.ingredient_product_id,
    quantity: String(ing.default_quantity ?? ""),
  };
}

export default function IngredientsEditor({ product }: IngredientsEditorProps) {
  const [ingredients, setIngredients] = useState<ProductIngredientWithProduct[]>([]);
  const [candidates, setCandidates] = useState<Product[]>([]);
  const [stockByProduct, setStockByProduct] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [recipe, products, stocks] = await Promise.all([
        recipeApi.list(product.id),
        productsApi.getAll(INGREDIENT_CANDIDATE_TYPES),
        stockApi.getAll(),
      ]);
      setIngredients(recipe);
      setCandidates(products.filter((p) => p.id !== product.id));
      const map: Record<string, number> = {};
      for (const s of stocks) map[s.product_id] = s.amount;
      setStockByProduct(map);
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo cargar la receta");
    } finally {
      setLoading(false);
    }
  }, [product.id]);

  useEffect(() => {
    load();
  }, [load]);

  const lines = ingredients.map(toLine);
  // A recipe line whose product left the candidate list still shows its name.
  const known = new Map(candidates.map((p) => [p.id, toCandidate(p)]));
  for (const ing of ingredients) {
    if (ing.ingredient_product && !known.has(ing.ingredient_product_id)) {
      known.set(ing.ingredient_product_id, toCandidate(ing.ingredient_product));
    }
  }

  const handleChange = async (next: RecipeLineDraft[]) => {
    const change = recipeChange(lines, next);
    if (!change) return;
    setBusy(true);
    setError("");
    try {
      if (change.kind === "add") {
        await recipeApi.add(product.id, {
          ingredient_product_id: change.line.ingredientId,
          quantity: change.line.quantity,
        });
      } else if (change.kind === "update") {
        await recipeApi.update(product.id, change.line.key, { quantity: change.line.quantity });
      } else {
        await recipeApi.remove(product.id, change.key);
      }
      await load();
    } catch (err) {
      const fallback = {
        add: "No se pudo agregar el ingrediente",
        update: "No se pudo actualizar la cantidad",
        remove: "No se pudo eliminar el ingrediente",
      }[change.kind];
      setError(err instanceof Error ? err.message : fallback);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      style={{
        padding: "1rem",
        backgroundColor: "var(--color-bg)",
        borderRadius: "var(--radius-sm)",
        border: "1px solid var(--color-border)",
        display: "flex",
        flexDirection: "column",
        gap: "1rem",
      }}
    >
      <h3
        style={{
          margin: 0,
          fontSize: "1rem",
          fontWeight: "600",
          color: "var(--color-text-secondary)",
        }}
      >
        Receta (Ingredientes)
      </h3>

      <div
        style={{
          padding: "0.75rem",
          backgroundColor: "var(--color-primary-light)",
          borderRadius: "var(--radius-sm)",
          border: "1px solid var(--color-primary)",
          color: "var(--color-text-primary)",
          fontSize: "0.875rem",
        }}
      >
        Al vender este producto compuesto se descuenta el stock de estos
        ingredientes. El producto compuesto no maneja stock propio.
      </div>

      {error && (
        <div
          role="alert"
          style={{
            padding: "0.75rem",
            backgroundColor: "var(--color-danger-light, #fde8e8)",
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--color-danger, #e02424)",
            color: "var(--color-danger, #e02424)",
            fontSize: "0.875rem",
          }}
        >
          {error}
        </div>
      )}

      {loading ? (
        <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
          Cargando receta...
        </p>
      ) : (
        <RecipeLinesFields
          value={lines}
          onChange={handleChange}
          candidates={Array.from(known.values())}
          disabled={busy}
          allowZero
          confirmEdits
          onInvalid={setError}
          lineNote={(line, unit) => `Stock: ${stockByProduct[line.ingredientId] ?? 0} ${unit}`}
        />
      )}
    </div>
  );
}
