"use client";

import { useState, useEffect, useCallback, type CSSProperties } from "react";
import type { Product, ProductType, PlateOptionSlot } from "@/types/product";
import { recipeApi } from "@/lib/api/recipe";
import { productsApi } from "@/lib/api/products";
import {
  draftKey,
  draftsFromSaved,
  draftsToRequest,
  move,
  newChoice,
  newSlot,
  removeChoice,
  setDefaultChoice,
  validateDrafts,
  type ChoiceDraft,
  type SlotDraft,
} from "@/lib/products/optionSlots";
import { Select, Input, Button } from "@/components/ui";

interface OptionSlotsEditorProps {
  // The persisted composite plate whose option slots we are editing.
  product: Product;
}

const BUNDLE_CANDIDATE_TYPES: ProductType[] = ["INGREDIENT", "BOTH", "COMPOSITE"];

const panel: CSSProperties = {
  padding: "1rem",
  backgroundColor: "var(--color-bg)",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--color-border)",
  display: "flex",
  flexDirection: "column",
  gap: "1rem",
};

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

const hint: CSSProperties = {
  margin: 0,
  color: "var(--color-text-secondary)",
  fontSize: "0.75rem",
  lineHeight: 1.4,
};

function saveErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : "No se pudieron guardar las opciones";
}

export default function OptionSlotsEditor({ product }: OptionSlotsEditorProps) {
  const [saved, setSaved] = useState<PlateOptionSlot[]>([]);
  const [drafts, setDrafts] = useState<SlotDraft[]>([]);
  const [candidates, setCandidates] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    try {
      const [slots, products] = await Promise.all([
        recipeApi.listOptionSlots(product.id),
        productsApi.getAll(BUNDLE_CANDIDATE_TYPES),
      ]);
      setSaved(slots);
      setDrafts(draftsFromSaved(slots));
      setCandidates(products.filter((p) => p.id !== product.id));
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "No se pudieron cargar las opciones",
      );
    } finally {
      setLoading(false);
    }
  }, [product.id]);

  useEffect(() => {
    load();
  }, [load]);

  const dirty =
    JSON.stringify(draftsToRequest(drafts)) !==
    JSON.stringify(draftsToRequest(draftsFromSaved(saved)));

  const updateSlot = (slotKey: string, fn: (s: SlotDraft) => SlotDraft) => {
    setNotice("");
    setDrafts((prev) => prev.map((s) => (s.key === slotKey ? fn(s) : s)));
  };

  const updateChoice = (
    slotKey: string,
    choiceKey: string,
    fn: (c: ChoiceDraft) => ChoiceDraft,
  ) =>
    updateSlot(slotKey, (s) => ({
      ...s,
      choices: s.choices.map((c) => (c.key === choiceKey ? fn(c) : c)),
    }));

  const handleSave = async () => {
    const problem = validateDrafts(drafts, product.id);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const result = await recipeApi.replaceOptionSlots(
        product.id,
        draftsToRequest(drafts),
      );
      setSaved(result);
      setDrafts(draftsFromSaved(result));
      setNotice("Opciones guardadas. Llegan a la caja en la próxima sincronización.");
    } catch (err) {
      setError(saveErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const handleDiscard = () => {
    setError("");
    setNotice("");
    setDrafts(draftsFromSaved(saved));
  };

  return (
    <div style={panel}>
      <h3
        style={{
          margin: 0,
          fontSize: "1rem",
          fontWeight: "600",
          color: "var(--color-text-secondary)",
        }}
      >
        Opciones del plato
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
        Cada grupo es una elección única (p. ej. Arroz: blanco <em>o</em> de
        coco) con una opción predeterminada. Cada opción descuenta su paquete
        de ingredientes; sin ingredientes significa &ldquo;sin&rdquo;. Las
        opciones nunca cambian el precio.
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

      {notice && (
        <p style={{ ...hint, color: "var(--color-primary)" }}>{notice}</p>
      )}

      {loading ? (
        <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
          Cargando opciones...
        </p>
      ) : (
        <>
          {drafts.length === 0 && (
            <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
              Este plato no tiene grupos de opciones.
            </p>
          )}

          {drafts.map((slot, slotIndex) => (
            <div key={slot.key} style={card}>
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
                    updateSlot(slot.key, (s) => ({ ...s, name: e.target.value }))
                  }
                  placeholder="Acompañante"
                  disabled={busy}
                />
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Subir grupo"
                  onClick={() => setDrafts((prev) => move(prev, slotIndex, -1))}
                  disabled={busy || slotIndex === 0}
                >
                  ↑
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  aria-label="Bajar grupo"
                  onClick={() => setDrafts((prev) => move(prev, slotIndex, 1))}
                  disabled={busy || slotIndex === drafts.length - 1}
                >
                  ↓
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() =>
                    setDrafts((prev) => prev.filter((s) => s.key !== slot.key))
                  }
                  disabled={busy}
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
                          updateSlot(slot.key, (s) => setDefaultChoice(s, choice.key))
                        }
                        disabled={busy}
                      />
                      Predet.
                    </label>
                    <Input
                      aria-label="Nombre de la opción"
                      value={choice.name}
                      onChange={(e) =>
                        updateChoice(slot.key, choice.key, (c) => ({
                          ...c,
                          name: e.target.value,
                        }))
                      }
                      placeholder="Papas"
                      disabled={busy}
                    />
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Subir opción"
                      onClick={() =>
                        updateSlot(slot.key, (s) => ({
                          ...s,
                          choices: move(s.choices, choiceIndex, -1),
                        }))
                      }
                      disabled={busy || choiceIndex === 0}
                    >
                      ↑
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label="Bajar opción"
                      onClick={() =>
                        updateSlot(slot.key, (s) => ({
                          ...s,
                          choices: move(s.choices, choiceIndex, 1),
                        }))
                      }
                      disabled={busy || choiceIndex === slot.choices.length - 1}
                    >
                      ↓
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        updateSlot(slot.key, (s) => removeChoice(s, choice.key))
                      }
                      disabled={busy || slot.choices.length === 1}
                    >
                      Quitar
                    </Button>
                  </div>

                  {choice.items.length === 0 && (
                    <p style={hint}>Sin ingredientes: el plato se sirve sin este grupo.</p>
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
                          updateChoice(slot.key, choice.key, (c) => ({
                            ...c,
                            items: c.items.map((it) =>
                              it.key === item.key
                                ? { ...it, ingredientProductId: e.target.value }
                                : it,
                            ),
                          }))
                        }
                        disabled={busy}
                      >
                        <option value="">Selecciona un ingrediente</option>
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
                          updateChoice(slot.key, choice.key, (c) => ({
                            ...c,
                            items: c.items.map((it) =>
                              it.key === item.key
                                ? { ...it, quantity: e.target.value }
                                : it,
                            ),
                          }))
                        }
                        disabled={busy}
                      />
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label="Quitar ingrediente"
                        onClick={() =>
                          updateChoice(slot.key, choice.key, (c) => ({
                            ...c,
                            items: c.items.filter((it) => it.key !== item.key),
                          }))
                        }
                        disabled={busy}
                      >
                        ✕
                      </Button>
                    </div>
                  ))}

                  <button
                    type="button"
                    style={linkButton}
                    onClick={() =>
                      updateChoice(slot.key, choice.key, (c) => ({
                        ...c,
                        items: [
                          ...c.items,
                          { key: draftKey(), ingredientProductId: "", quantity: "1" },
                        ],
                      }))
                    }
                    disabled={busy}
                  >
                    + Ingrediente
                  </button>
                </div>
              ))}

              <button
                type="button"
                style={linkButton}
                onClick={() =>
                  updateSlot(slot.key, (s) => ({
                    ...s,
                    choices: [...s.choices, newChoice(false)],
                  }))
                }
                disabled={busy}
              >
                + Opción
              </button>
            </div>
          ))}

          <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem" }}>
            <Button
              variant="secondary"
              onClick={() => {
                setNotice("");
                setDrafts((prev) => [...prev, newSlot()]);
              }}
              disabled={busy}
            >
              + Grupo
            </Button>
            <div style={{ flex: 1 }} />
            <Button
              variant="secondary"
              onClick={handleDiscard}
              disabled={busy || !dirty}
            >
              Descartar cambios
            </Button>
            <Button onClick={handleSave} disabled={busy || !dirty}>
              {busy ? "Guardando..." : "Guardar opciones"}
            </Button>
          </div>

          <p style={hint}>
            Quitar una opción o un grupo impide pedirlo de ahora en adelante;
            los pedidos ya tomados lo siguen mostrando.
          </p>
        </>
      )}
    </div>
  );
}
