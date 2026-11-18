import { test } from "node:test";
import assert from "node:assert/strict";
import { deriveUpdateBanner } from "./update.ts";
import type { EdgeUpdateResult, EdgeUpdateStatus } from "@/types/edge";

const base: EdgeUpdateStatus = {
  enabled: true,
  installed: { version: "v1.0.0" },
  available: null,
  requested: null,
  in_progress: null,
  last_result: null,
};

function withResult(
  outcome: EdgeUpdateResult["outcome"],
  message?: string,
): EdgeUpdateStatus {
  return {
    ...base,
    last_result: {
      version: "v1.0.1",
      outcome,
      message,
      finished_at: "2026-10-19T15:10:00-05:00",
    },
  };
}

test("hidden when disabled or unknown", () => {
  assert.equal(deriveUpdateBanner(null, null, null), null);
  assert.equal(deriveUpdateBanner({ enabled: false }, null, null), null);
  assert.equal(deriveUpdateBanner(base, null, null), null);
});

test("available release", () => {
  const banner = deriveUpdateBanner(
    { ...base, available: { version: "v1.0.1" } },
    null,
    null,
  );
  assert.equal(banner?.kind, "available");
  assert.equal(banner?.message, "Versión v1.0.1 disponible");
});

test("requested and in progress both show updating", () => {
  const expected =
    "Actualizando a v1.0.1… el sistema se reiniciará en unos minutos.";
  for (const field of ["requested", "in_progress"] as const) {
    const banner = deriveUpdateBanner(
      { ...base, available: { version: "v1.0.1" }, [field]: { version: "v1.0.1" } },
      null,
      null,
    );
    assert.equal(banner?.kind, "updating");
    assert.equal(banner?.message, expected);
  }
});

test("a pending request keeps updating while the backend is unreachable", () => {
  const pending = { version: "v1.0.1", baselineFinishedAt: null };
  assert.equal(deriveUpdateBanner(null, pending, null)?.kind, "updating");
});

test("a pending request ends when a new result appears", () => {
  const pending = { version: "v1.0.1", baselineFinishedAt: null };
  const banner = deriveUpdateBanner(withResult("success"), pending, null);
  assert.equal(banner?.message, "Actualizado a v1.0.1");
});

test("result messages", () => {
  const cases: [EdgeUpdateResult["outcome"], string, string?][] = [
    ["success", "Actualizado a v1.0.1"],
    ["refused", "No se pudo actualizar: Outbox not synced", "Outbox not synced"],
    ["rolled_back", "La actualización falló; se restauró la versión anterior."],
    ["rollback_failed", "Error grave en la actualización. Contacte a soporte."],
  ];
  for (const [outcome, expected, message] of cases) {
    const banner = deriveUpdateBanner(withResult(outcome, message), null, null);
    assert.equal(banner?.kind, "result");
    assert.equal(banner?.message, expected);
  }
});

test("refused offers a retry when the release is still staged", () => {
  const status = { ...withResult("refused", "low disk"), available: { version: "v1.0.1" } };
  const banner = deriveUpdateBanner(status, null, null);
  assert.equal(banner?.kind === "result" && banner.retryVersion, "v1.0.1");
});

test("drill rollbacks show nothing", () => {
  assert.equal(deriveUpdateBanner(withResult("drill_rolled_back"), null, null), null);
});

test("a dismissed result falls back to the available release", () => {
  const status = { ...withResult("success"), available: { version: "v1.0.2" } };
  const banner = deriveUpdateBanner(status, null, "2026-10-19T15:10:00-05:00");
  assert.equal(banner?.message, "Versión v1.0.2 disponible");
  assert.equal(
    deriveUpdateBanner(withResult("success"), null, "2026-10-19T15:10:00-05:00"),
    null,
  );
});
