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

  // Quantity 0 is allowed: a composite may carry a "swappable" ingredient that
  // isn't on the standard plate (default 0) and is only added if the customer
  // asks — the same shape as a side dish with default 0. The backend accepts 0.
  const validQuantity = (value: string): boolean => {
    const qty = parseFloat(value);
    return value.trim() !== "" && !isNaN(qty) && qty >= 0;
  };

  const handleAdd = async () => {
    if (!newIngredientId) {
      setError("Selecciona un ingrediente");
      return;
    }
    if (!validQuantity(newQuantity)) {
      setError("La cantidad debe ser un número igual o mayor a 0");
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
      setError("La cantidad debe ser un número igual o mayor a 0");
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

  const handleConfigureSideDish = async (
    ingredientId: string,
    def: number,
    min: number,
    max: number,
  ) => {
    // Client-side guard mirrors the backend: 0 <= min <= default <= max.
    if (!(0 <= min && min <= def && def <= max)) {
      setError("Los límites deben cumplir 0 ≤ mín ≤ predet ≤ máx");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await recipeApi.configureSideDish(product.id, ingredientId, {
        default_quantity: def,
        min_quantity: min,
        max_quantity: max,
      });
      await load();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No se pudo guardar la guarnición",
      );
    } finally {
      setBusy(false);
    }
  };

  const handleClearSideDish = async (ingredientId: string) => {
    setBusy(true);
    setError("");
    try {
      await recipeApi.clearSideDish(product.id, ingredientId);
      await load();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No se pudo quitar la guarnición",
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
                  onConfigureSideDish={(def, min, max) =>
                    handleConfigureSideDish(ing.id, def, min, max)
                  }
                  onClearSideDish={() => handleClearSideDish(ing.id)}
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
              min="0"
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
  onConfigureSideDish: (def: number, min: number, max: number) => void;
  onClearSideDish: () => void;
}

function toInt(value: string): number {
  const n = parseInt(value, 10);
  return isNaN(n) ? 0 : n;
}

function IngredientRow({
  ingredient,
  stock,
  disabled,
  onSave,
  onRemove,
  onConfigureSideDish,
  onClearSideDish,
}: IngredientRowProps) {
  const initialQty = String(ingredient.default_quantity ?? "");
  const [qty, setQty] = useState(initialQty);

  // Keep the input in sync when the recipe is reloaded after a save.
  useEffect(() => {
    setQty(String(ingredient.default_quantity ?? ""));
  }, [ingredient.default_quantity]);

  // Side-dish (guarnición) controls. When the row is already a side dish, seed
  // the inputs from its saved bounds; otherwise offer a sensible template.
  const isSideDish = ingredient.is_side_dish;
  const [showSideDish, setShowSideDish] = useState(isSideDish);
  const parsedDef = parseInt(ingredient.default_quantity ?? "", 10);
  const [sdDef, setSdDef] = useState(
    String(isSideDish && !isNaN(parsedDef) ? parsedDef : 1),
  );
  const [sdMin, setSdMin] = useState(
    String(isSideDish ? ingredient.min_quantity : 0),
  );
  const [sdMax, setSdMax] = useState(
    String(isSideDish ? ingredient.max_quantity : 2),
  );

  // Re-sync side-dish inputs when the recipe reloads after a save/clear.
  useEffect(() => {
    setShowSideDish(ingredient.is_side_dish);
    const def = parseInt(ingredient.default_quantity ?? "", 10);
    setSdDef(String(ingredient.is_side_dish && !isNaN(def) ? def : 1));
    setSdMin(String(ingredient.is_side_dish ? ingredient.min_quantity : 0));
    setSdMax(String(ingredient.is_side_dish ? ingredient.max_quantity : 2));
  }, [
    ingredient.is_side_dish,
    ingredient.default_quantity,
    ingredient.min_quantity,
    ingredient.max_quantity,
  ]);

  const unit = unitAbbrev(ingredient.ingredient_product?.unit_of_measure);
  const name =
    ingredient.ingredient_product?.name || ingredient.ingredient_product_id;
  const changed = qty !== initialQty;

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "0.5rem",
        padding: "0.5rem 0.75rem",
        backgroundColor: "var(--color-surface)",
        borderRadius: "var(--radius-sm)",
        border: "1px solid var(--color-border)",
      }}
    >
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "2fr 1fr 1fr auto auto",
          gap: "0.75rem",
          alignItems: "center",
        }}
      >
        <span style={{ color: "var(--color-text-primary)", fontWeight: 500 }}>
          {name}
          {isSideDish && (
            <span
              style={{
                marginLeft: "0.5rem",
                padding: "0.1rem 0.4rem",
                borderRadius: "var(--radius-sm)",
                backgroundColor: "var(--color-primary-light)",
                border: "1px solid var(--color-primary)",
                color: "var(--color-primary)",
                fontSize: "0.7rem",
                fontWeight: 600,
                verticalAlign: "middle",
              }}
            >
              Guarnición
            </span>
          )}
        </span>

        <div style={{ display: "flex", alignItems: "center", gap: "0.35rem" }}>
          <Input
            type="number"
            min="0"
            step="1"
            value={qty}
            onChange={(e) => setQty(e.target.value)}
            disabled={disabled}
          />
          <span
            style={{
              color: "var(--color-text-secondary)",
              fontSize: "0.875rem",
            }}
          >
            {unit}
          </span>
        </div>

        <span
          style={{
            color: "var(--color-text-secondary)",
            fontSize: "0.875rem",
          }}
        >
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

      {/* Side-dish (guarnición) controls */}
      {!showSideDish ? (
        <button
          type="button"
          onClick={() => setShowSideDish(true)}
          disabled={disabled}
          style={{
            alignSelf: "flex-start",
            background: "none",
            border: "none",
            padding: 0,
            cursor: disabled ? "default" : "pointer",
            color: "var(--color-primary)",
            fontSize: "0.8rem",
            fontWeight: 500,
          }}
        >
          + Marcar como guarnición
        </button>
      ) : (
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "0.5rem",
            padding: "0.5rem 0.75rem",
            backgroundColor: "var(--color-bg)",
            borderRadius: "var(--radius-sm)",
            border: "1px dashed var(--color-border)",
          }}
        >
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "flex-end",
              gap: "0.75rem",
            }}
          >
            <div style={{ width: "5rem" }}>
              <Input
                label="Predet."
                type="number"
                min="0"
                step="1"
                value={sdDef}
                onChange={(e) => setSdDef(e.target.value)}
                disabled={disabled}
              />
            </div>
            <div style={{ width: "5rem" }}>
              <Input
                label="Mín."
                type="number"
                min="0"
                step="1"
                value={sdMin}
                onChange={(e) => setSdMin(e.target.value)}
                disabled={disabled}
              />
            </div>
            <div style={{ width: "5rem" }}>
              <Input
                label="Máx."
                type="number"
                min="0"
                step="1"
                value={sdMax}
                onChange={(e) => setSdMax(e.target.value)}
                disabled={disabled}
              />
            </div>

            <Button
              type="button"
              onClick={() =>
                onConfigureSideDish(toInt(sdDef), toInt(sdMin), toInt(sdMax))
              }
              disabled={disabled}
            >
              Guardar guarnición
            </Button>

            {isSideDish ? (
              <Button
                type="button"
                variant="secondary"
                onClick={onClearSideDish}
                disabled={disabled}
              >
                Quitar guarnición
              </Button>
            ) : (
              <Button
                type="button"
                variant="secondary"
                onClick={() => setShowSideDish(false)}
                disabled={disabled}
              >
                Cancelar
              </Button>
            )}
          </div>

          <p
            style={{
              margin: 0,
              color: "var(--color-text-secondary)",
              fontSize: "0.75rem",
              lineHeight: 1.4,
            }}
          >
            El cliente puede ajustar esta guarnición entre mín. y máx. (números
            enteros). Predeterminado <strong>0</strong> = no viene en el plato
            estándar pero se puede agregar (p. ej. papas fritas 0/0/2). Las
            guarniciones nunca cambian el precio; mín./máx. son solo el límite.
          </p>
        </div>
      )}
    </div>
  );
}
