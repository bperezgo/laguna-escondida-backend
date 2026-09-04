"use client";

import { useState, useEffect, useCallback } from "react";
import type {
  Product,
  ProductType,
  ProductIngredientWithProduct,
  UnitOfMeasure,
} from "@/types/product";
import { UNITS_OF_MEASURE } from "@/types/product";
import { recipeApi } from "@/lib/api/recipe";
import { productsApi } from "@/lib/api/products";
import { stockApi } from "@/lib/api/stock";
import { Select, Input, Button } from "@/components/ui";

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

function unitAbbrev(unit?: UnitOfMeasure | string): string {
  if (!unit) return "";
  const match = UNITS_OF_MEASURE.find((u) => u.value === unit);
  // Prefer the short code in parentheses when available (e.g. "g"), else the raw unit.
  return match ? match.value : String(unit);
}

export default function IngredientsEditor({ product }: IngredientsEditorProps) {
  const [ingredients, setIngredients] = useState<
    ProductIngredientWithProduct[]
  >([]);
  const [candidates, setCandidates] = useState<Product[]>([]);
  const [stockByProduct, setStockByProduct] = useState<Record<string, number>>(
    {},
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Add-ingredient form state
  const [newIngredientId, setNewIngredientId] = useState("");
  const [newQuantity, setNewQuantity] = useState("");

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
      setError(
        err instanceof Error ? err.message : "No se pudo cargar la receta",
      );
    } finally {
      setLoading(false);
    }
  }, [product.id]);

  useEffect(() => {
    load();
  }, [load]);

  const usedIds = new Set(ingredients.map((i) => i.ingredient_product_id));
  const availableCandidates = candidates.filter((c) => !usedIds.has(c.id));

  const validQuantity = (value: string): boolean => {
    const qty = parseFloat(value);
    return value.trim() !== "" && !isNaN(qty) && qty > 0;
  };

  const handleAdd = async () => {
    if (!newIngredientId) {
      setError("Selecciona un ingrediente");
      return;
    }
    if (!validQuantity(newQuantity)) {
      setError("La cantidad debe ser un número mayor a 0");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await recipeApi.add(product.id, {
        ingredient_product_id: newIngredientId,
        quantity: newQuantity.trim(),
      });
      setNewIngredientId("");
      setNewQuantity("");
      await load();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No se pudo agregar el ingrediente",
      );
    } finally {
      setBusy(false);
    }
  };

  const handleUpdate = async (ingredientId: string, quantity: string) => {
    if (!validQuantity(quantity)) {
      setError("La cantidad debe ser un número mayor a 0");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await recipeApi.update(product.id, ingredientId, {
        quantity: quantity.trim(),
      });
      await load();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No se pudo actualizar la cantidad",
      );
    } finally {
      setBusy(false);
    }
  };

  const handleRemove = async (ingredientId: string) => {
    setBusy(true);
    setError("");
    try {
      await recipeApi.remove(product.id, ingredientId);
      await load();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No se pudo eliminar el ingrediente",
      );
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
        <>
          {ingredients.length === 0 ? (
            <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
              Aún no hay ingredientes en la receta.
            </p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
              {ingredients.map((ing) => (
                <IngredientRow
                  key={ing.id}
                  ingredient={ing}
                  stock={stockByProduct[ing.ingredient_product_id]}
                  disabled={busy}
                  onSave={(qty) => handleUpdate(ing.id, qty)}
                  onRemove={() => handleRemove(ing.id)}
                />
              ))}
            </div>
          )}

          {/* Add ingredient row */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "2fr 1fr auto",
              gap: "0.75rem",
              alignItems: "end",
            }}
          >
            <Select
              label="Ingrediente"
              value={newIngredientId}
              onChange={(e) => setNewIngredientId(e.target.value)}
              disabled={busy || availableCandidates.length === 0}
            >
              <option value="">
                {availableCandidates.length === 0
                  ? "No hay ingredientes disponibles"
                  : "Selecciona un ingrediente"}
              </option>
              {availableCandidates.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({unitAbbrev(c.unit_of_measure)})
                </option>
              ))}
            </Select>

            <Input
              label="Cantidad"
              type="number"
              min="1"
              step="1"
              value={newQuantity}
              onChange={(e) => setNewQuantity(e.target.value)}
              placeholder="0"
              disabled={busy}
            />

            <Button
              type="button"
              onClick={handleAdd}
              disabled={busy || !newIngredientId || !newQuantity}
            >
              Agregar
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

interface IngredientRowProps {
  ingredient: ProductIngredientWithProduct;
  stock?: number;
  disabled: boolean;
  onSave: (quantity: string) => void;
  onRemove: () => void;
}

function IngredientRow({
  ingredient,
  stock,
  disabled,
  onSave,
  onRemove,
}: IngredientRowProps) {
  const initialQty = String(ingredient.quantity ?? "");
  const [qty, setQty] = useState(initialQty);

  // Keep the input in sync when the recipe is reloaded after a save.
  useEffect(() => {
    setQty(String(ingredient.quantity ?? ""));
  }, [ingredient.quantity]);

  const unit = unitAbbrev(ingredient.ingredient_product?.unit_of_measure);
  const name =
    ingredient.ingredient_product?.name || ingredient.ingredient_product_id;
  const changed = qty !== initialQty;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "2fr 1fr 1fr auto auto",
        gap: "0.75rem",
        alignItems: "center",
        padding: "0.5rem 0.75rem",
        backgroundColor: "var(--color-surface)",
        borderRadius: "var(--radius-sm)",
        border: "1px solid var(--color-border)",
      }}
    >
      <span style={{ color: "var(--color-text-primary)", fontWeight: 500 }}>
        {name}
      </span>

      <div style={{ display: "flex", alignItems: "center", gap: "0.35rem" }}>
        <Input
          type="number"
          min="1"
          step="1"
          value={qty}
          onChange={(e) => setQty(e.target.value)}
          disabled={disabled}
        />
        <span style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
          {unit}
        </span>
      </div>

      <span style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
        Stock: {stock ?? 0} {unit}
      </span>

      <Button
        type="button"
        variant="secondary"
        onClick={() => onSave(qty)}
        disabled={disabled || !changed}
      >
        Guardar
      </Button>

      <Button
        type="button"
        variant="secondary"
        onClick={onRemove}
        disabled={disabled}
      >
        Eliminar
      </Button>
    </div>
  );
}
