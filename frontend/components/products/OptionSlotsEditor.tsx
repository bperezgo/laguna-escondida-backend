"use client";

import { useState, useEffect, useCallback, type CSSProperties } from "react";
import type { Product, ProductType, PlateOptionSlot } from "@/types/product";
import { recipeApi } from "@/lib/api/recipe";
import { productsApi } from "@/lib/api/products";
import {
  draftsFromSaved,
  draftsToRequest,
  findDraftProblem,
  type SlotDraft,
} from "@/lib/products/optionSlots";
import { Button } from "@/components/ui";
import OptionSlotsFields, { slotHint as hint } from "@/components/products/OptionSlotsFields";

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
  const [markedSlot, setMarkedSlot] = useState<string | undefined>();

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

  const handleSave = async () => {
    const problem = findDraftProblem(drafts, product.id);
    setMarkedSlot(problem?.slotKey);
    if (problem) {
      setError(problem.message);
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
    setMarkedSlot(undefined);
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
          <OptionSlotsFields
            value={drafts}
            onChange={(next) => {
              setNotice("");
              setDrafts(next);
            }}
            candidates={candidates.map((p) => ({ id: p.id, name: p.name, unit: p.unit_of_measure }))}
            disabled={busy}
            markedSlotKey={markedSlot}
          />

          <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem" }}>
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
