"use client";

import { useState } from "react";
import { Button, Modal } from "@/components/ui";
import RecipeLinesFields from "@/components/products/RecipeLinesFields";
import OptionSlotsFields from "@/components/products/OptionSlotsFields";
import type { SlotDraft } from "@/lib/products/optionSlots";
import type { RecipeLineDraft } from "@/lib/products/recipeLines";
import { panelProblem, type PanelProblem } from "@/lib/catalogGrid/panel";
import type { Candidate, GridRow } from "@/lib/catalogGrid/state";
import type { PanelProblem as ServerPanelProblem } from "@/lib/catalogGrid/check";

interface PlatePanelProps {
  plate: GridRow;
  candidates: Candidate[];
  serverProblems: ServerPanelProblem[];
  onApply: (recipe: RecipeLineDraft[], slots: SlotDraft[]) => void;
  onCancel: () => void;
}

const sectionTitle = {
  margin: 0,
  fontSize: "1rem",
  fontWeight: 600,
  color: "var(--color-text-secondary)",
} as const;

const alert = {
  padding: "0.75rem",
  backgroundColor: "var(--color-danger-light, #fde8e8)",
  borderRadius: "var(--radius-sm)",
  border: "1px solid var(--color-danger, #e02424)",
  color: "var(--color-danger, #e02424)",
  fontSize: "0.875rem",
} as const;

export default function PlatePanel({ plate, candidates, serverProblems, onApply, onCancel }: PlatePanelProps) {
  const [recipe, setRecipe] = useState<RecipeLineDraft[]>(plate.recipe);
  const [slots, setSlots] = useState<SlotDraft[]>(plate.slots);
  const [problem, setProblem] = useState<PanelProblem | null>(null);
  const [inputError, setInputError] = useState("");

  const firstServer = serverProblems[0]?.panel;
  const markedRecipe = problem?.kind === "recipe" ? problem.key : firstServer?.kind === "recipe" ? firstServer.key : undefined;
  const markedSlot = problem?.kind === "slot" ? problem.key : firstServer?.kind === "slot" ? firstServer.key : undefined;

  // Closing by any route runs the slot rules first, so a slot without a default keeps it open.
  const tryClose = () => {
    const found = panelProblem(plate.id, recipe, slots);
    setProblem(found);
    if (!found) onApply(recipe, slots);
  };

  return (
    <Modal
      open
      onClose={tryClose}
      closeOnOverlay={false}
      size="lg"
      title={`Receta y opciones: ${plate.name.trim() || "plato sin nombre"}`}
      footer={
        <div style={{ display: "flex", gap: "0.5rem", justifyContent: "flex-end" }}>
          <Button variant="secondary" onClick={onCancel}>
            Descartar cambios
          </Button>
          <Button onClick={tryClose}>Listo</Button>
        </div>
      }
    >
      <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
        {serverProblems.length > 0 && (
          <div role="alert" style={alert}>
            {serverProblems.map((p, i) => (
              <div key={i}>{p.message}</div>
            ))}
          </div>
        )}
        {(problem || inputError) && (
          <div role="alert" style={alert}>
            {problem?.message ?? inputError}
          </div>
        )}

        <h3 style={sectionTitle}>Receta</h3>
        <RecipeLinesFields
          value={recipe}
          onChange={(next) => {
            setInputError("");
            setProblem(null);
            setRecipe(next);
          }}
          candidates={candidates}
          allowZero={false}
          onInvalid={setInputError}
          markedKey={markedRecipe}
        />

        <h3 style={sectionTitle}>Opciones del plato</h3>
        <OptionSlotsFields
          value={slots}
          onChange={(next) => {
            setProblem(null);
            setSlots(next);
          }}
          candidates={candidates}
          markedSlotKey={markedSlot}
        />
      </div>
    </Modal>
  );
}
