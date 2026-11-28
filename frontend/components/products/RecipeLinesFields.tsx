"use client";

import { useState, type ReactNode } from "react";
import { Select, Input, Button } from "@/components/ui";
import {
  addLine,
  availableCandidates,
  quantityMessage,
  removeLine,
  setLineQuantity,
  validRecipeQuantity,
  type IngredientCandidate,
  type RecipeLineDraft,
} from "@/lib/products/recipeLines";

interface RecipeLinesFieldsProps {
  value: RecipeLineDraft[];
  onChange: (next: RecipeLineDraft[]) => void;
  candidates: IngredientCandidate[];
  disabled?: boolean;
  allowZero: boolean;
  // The product page saves each quantity with a button; the grid applies it as typed.
  confirmEdits?: boolean;
  onInvalid?: (message: string) => void;
  lineNote?: (line: RecipeLineDraft, unit: string) => ReactNode;
  markedKey?: string;
}

export default function RecipeLinesFields({
  value,
  onChange,
  candidates,
  disabled = false,
  allowZero,
  confirmEdits = false,
  onInvalid,
  lineNote,
  markedKey,
}: RecipeLinesFieldsProps) {
  const [newIngredientId, setNewIngredientId] = useState("");
  const [newQuantity, setNewQuantity] = useState("");
  const byId = new Map(candidates.map((c) => [c.id, c]));
  const available = availableCandidates(value, candidates);

  const handleAdd = () => {
    if (!newIngredientId) {
      onInvalid?.("Selecciona un ingrediente");
      return;
    }
    if (!validRecipeQuantity(newQuantity, allowZero)) {
      onInvalid?.(quantityMessage(allowZero));
      return;
    }
    onChange(addLine(value, newIngredientId, newQuantity));
    setNewIngredientId("");
    setNewQuantity("");
  };

  const handleQuantity = (key: string, quantity: string) => {
    if (!validRecipeQuantity(quantity, allowZero)) {
      onInvalid?.(quantityMessage(allowZero));
      return;
    }
    onChange(setLineQuantity(value, key, quantity));
  };

  return (
    <>
      {value.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
          Aún no hay ingredientes en la receta.
        </p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          {value.map((line) => {
            const candidate = byId.get(line.ingredientId);
            return (
              <RecipeLineRow
                // Remounts with the saved quantity after a reload, dropping an unsaved edit.
                key={`${line.key}:${line.quantity}`}
                name={candidate?.name ?? line.ingredientId}
                unit={candidate?.unit ?? ""}
                quantity={line.quantity}
                note={lineNote?.(line, candidate?.unit ?? "")}
                disabled={disabled}
                confirmEdits={confirmEdits}
                marked={markedKey === line.key}
                onQuantity={(qty) => handleQuantity(line.key, qty)}
                onRemove={() => onChange(removeLine(value, line.key))}
              />
            );
          })}
        </div>
      )}

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
          disabled={disabled || available.length === 0}
        >
          <option value="">
            {available.length === 0
              ? "No hay ingredientes disponibles"
              : "Selecciona un ingrediente"}
          </option>
          {available.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
              {c.unit ? ` (${c.unit})` : ""}
            </option>
          ))}
        </Select>

        <Input
          label="Cantidad"
          type="number"
          min="0"
          step="any"
          value={newQuantity}
          onChange={(e) => setNewQuantity(e.target.value)}
          placeholder="0"
          disabled={disabled}
        />

        <Button
          type="button"
          onClick={handleAdd}
          disabled={disabled || !newIngredientId || !newQuantity}
        >
          Agregar
        </Button>
      </div>
    </>
  );
}

interface RecipeLineRowProps {
  name: string;
  unit: string;
  quantity: string;
  note?: ReactNode;
  disabled: boolean;
  confirmEdits: boolean;
  marked: boolean;
  onQuantity: (quantity: string) => void;
  onRemove: () => void;
}

function RecipeLineRow({
  name,
  unit,
  quantity,
  note,
  disabled,
  confirmEdits,
  marked,
  onQuantity,
  onRemove,
}: RecipeLineRowProps) {
  const [qty, setQty] = useState(quantity);

  const changed = qty !== quantity;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: confirmEdits ? "2fr 1fr 1fr auto auto" : "2fr 1fr 1fr auto",
        gap: "0.75rem",
        alignItems: "center",
        padding: "0.5rem 0.75rem",
        backgroundColor: "var(--color-surface)",
        borderRadius: "var(--radius-sm)",
        border: marked
          ? "1px solid var(--color-danger, #e02424)"
          : "1px solid var(--color-border)",
      }}
    >
      <span style={{ color: "var(--color-text-primary)", fontWeight: 500 }}>{name}</span>

      <div style={{ display: "flex", alignItems: "center", gap: "0.35rem" }}>
        <Input
          aria-label={`Cantidad de ${name}`}
          type="number"
          min="0"
          step="any"
          value={qty}
          onChange={(e) => setQty(e.target.value)}
          onBlur={() => {
            if (!confirmEdits && changed) onQuantity(qty);
          }}
          disabled={disabled}
        />
        <span style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>{unit}</span>
      </div>

      <span style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>{note}</span>

      {confirmEdits && (
        <Button
          type="button"
          variant="secondary"
          onClick={() => onQuantity(qty)}
          disabled={disabled || !changed}
        >
          Guardar
        </Button>
      )}

      <Button type="button" variant="secondary" onClick={onRemove} disabled={disabled}>
        Eliminar
      </Button>
    </div>
  );
}
