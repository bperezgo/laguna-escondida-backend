"use client";

import type { CSSProperties } from "react";
import {
  addItem,
  move,
  newChoice,
  newSlot,
  removeChoice,
  removeItem,
  setDefaultChoice,
  updateChoiceIn,
  updateItem,
  updateSlotIn,
  type SlotDraft,
} from "@/lib/products/optionSlots";
import type { IngredientCandidate } from "@/lib/products/recipeLines";
import { Select, Input, Button } from "@/components/ui";

interface OptionSlotsFieldsProps {
  value: SlotDraft[];
  onChange: (next: SlotDraft[]) => void;
  candidates: IngredientCandidate[];
  disabled?: boolean;
  markedSlotKey?: string;
}

const card: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: "0.75rem",
  padding: "0.75rem",
  backgroundColor: "var(--color-surface)",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--color-border)",
};

const choiceBox: CSSProperties = {
  display: "flex",
  flexDirection: "column",
  gap: "0.5rem",
  padding: "0.5rem 0.75rem",
  backgroundColor: "var(--color-bg)",
  borderRadius: "var(--radius-sm)",
  border: "1px dashed var(--color-border)",
};

const linkButton: CSSProperties = {
  alignSelf: "flex-start",
  background: "none",
  border: "none",
  padding: 0,
  cursor: "pointer",
  color: "var(--color-primary)",
  fontSize: "0.8rem",
  fontWeight: 500,
};

export const slotHint: CSSProperties = {
  margin: 0,
  color: "var(--color-text-secondary)",
  fontSize: "0.75rem",
  lineHeight: 1.4,
};

export default function OptionSlotsFields({
  value,
  onChange,
  candidates,
  disabled = false,
  markedSlotKey,
}: OptionSlotsFieldsProps) {
  const known = new Set(candidates.map((c) => c.id));

  return (
    <>
      {value.length === 0 && (
        <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
          Este plato no tiene grupos de opciones.
        </p>
      )}

      {value.map((slot, slotIndex) => (
        <div
          key={slot.key}
          style={
            slot.key === markedSlotKey
              ? { ...card, border: "1px solid var(--color-danger, #e02424)" }
              : card
          }
          aria-invalid={slot.key === markedSlotKey || undefined}
        >
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr auto auto auto",
              gap: "0.5rem",
              alignItems: "end",
            }}
          >
            <Input
              label="Grupo"
              value={slot.name}
              onChange={(e) =>
                onChange(updateSlotIn(value, slot.key, (s) => ({ ...s, name: e.target.value })))
              }
              placeholder="Acompañante"
              disabled={disabled}
            />
            <Button
              variant="ghost"
              size="sm"
              aria-label="Subir grupo"
              onClick={() => onChange(move(value, slotIndex, -1))}
              disabled={disabled || slotIndex === 0}
            >
              ↑
            </Button>
            <Button
              variant="ghost"
              size="sm"
              aria-label="Bajar grupo"
              onClick={() => onChange(move(value, slotIndex, 1))}
              disabled={disabled || slotIndex === value.length - 1}
            >
              ↓
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => onChange(value.filter((s) => s.key !== slot.key))}
              disabled={disabled}
            >
              Eliminar grupo
            </Button>
          </div>

          {slot.choices.map((choice, choiceIndex) => (
            <div key={choice.key} style={choiceBox}>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "auto 1fr auto auto auto",
                  gap: "0.5rem",
                  alignItems: "center",
                }}
              >
                <label
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.25rem",
                    fontSize: "0.75rem",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  <input
                    type="radio"
                    name={`default-${slot.key}`}
                    checked={choice.isDefault}
                    onChange={() =>
                      onChange(updateSlotIn(value, slot.key, (s) => setDefaultChoice(s, choice.key)))
                    }
                    disabled={disabled}
                  />
                  Predet.
                </label>
                <Input
                  aria-label="Nombre de la opción"
                  value={choice.name}
                  onChange={(e) =>
                    onChange(
                      updateChoiceIn(value, slot.key, choice.key, (c) => ({
                        ...c,
                        name: e.target.value,
                      })),
                    )
                  }
                  placeholder="Papas"
                  disabled={disabled}
                />
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Subir opción"
                  onClick={() =>
                    onChange(
                      updateSlotIn(value, slot.key, (s) => ({
                        ...s,
                        choices: move(s.choices, choiceIndex, -1),
                      })),
                    )
                  }
                  disabled={disabled || choiceIndex === 0}
                >
                  ↑
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Bajar opción"
                  onClick={() =>
                    onChange(
                      updateSlotIn(value, slot.key, (s) => ({
                        ...s,
                        choices: move(s.choices, choiceIndex, 1),
                      })),
                    )
                  }
                  disabled={disabled || choiceIndex === slot.choices.length - 1}
                >
                  ↓
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() =>
                    onChange(updateSlotIn(value, slot.key, (s) => removeChoice(s, choice.key)))
                  }
                  disabled={disabled || slot.choices.length === 1}
                >
                  Quitar
                </Button>
              </div>

              {choice.items.length === 0 && (
                <p style={slotHint}>Sin ingredientes: el plato se sirve sin este grupo.</p>
              )}

              {choice.items.map((item) => (
                <div
                  key={item.key}
                  style={{
                    display: "grid",
                    gridTemplateColumns: "2fr 6rem auto",
                    gap: "0.5rem",
                    alignItems: "center",
                  }}
                >
                  <Select
                    aria-label="Ingrediente"
                    value={item.ingredientProductId}
                    onChange={(e) =>
                      onChange(
                        updateChoiceIn(value, slot.key, choice.key, (c) =>
                          updateItem(c, item.key, { ingredientProductId: e.target.value }),
                        ),
                      )
                    }
                    disabled={disabled}
                  >
                    <option value="">Selecciona un ingrediente</option>
                    {item.ingredientProductId && !known.has(item.ingredientProductId) && (
                      <option value={item.ingredientProductId}>Ingrediente no disponible</option>
                    )}
                    {candidates.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </Select>
                  <Input
                    aria-label="Cantidad"
                    type="number"
                    min="1"
                    step="1"
                    value={item.quantity}
                    onChange={(e) =>
                      onChange(
                        updateChoiceIn(value, slot.key, choice.key, (c) =>
                          updateItem(c, item.key, { quantity: e.target.value }),
                        ),
                      )
                    }
                    disabled={disabled}
                  />
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label="Quitar ingrediente"
                    onClick={() =>
                      onChange(
                        updateChoiceIn(value, slot.key, choice.key, (c) => removeItem(c, item.key)),
                      )
                    }
                    disabled={disabled}
                  >
                    ✕
                  </Button>
                </div>
              ))}

              <button
                type="button"
                style={linkButton}
                onClick={() => onChange(updateChoiceIn(value, slot.key, choice.key, addItem))}
                disabled={disabled}
              >
                + Ingrediente
              </button>
            </div>
          ))}

          <button
            type="button"
            style={linkButton}
            onClick={() =>
              onChange(
                updateSlotIn(value, slot.key, (s) => ({
                  ...s,
                  choices: [...s.choices, newChoice(false)],
                })),
              )
            }
            disabled={disabled}
          >
            + Opción
          </button>
        </div>
      ))}

      <div>
        <Button
          variant="secondary"
          onClick={() => onChange([...value, newSlot()])}
          disabled={disabled}
        >
          + Grupo
        </Button>
      </div>
    </>
  );
}
