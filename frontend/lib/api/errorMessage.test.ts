import { test } from "node:test";
import assert from "node:assert/strict";
import { apiErrorMessage } from "./errorMessage.ts";

test("a plan refusal reads as an upgrade notice naming the plan", () => {
  const message = apiErrorMessage(
    403,
    { error: "Your plan does not include this feature", code: "plan_required", required_plan: "full" },
    null
  );
  assert.match(message, /Tu plan no incluye esta función/);
  assert.match(message, /Completo/);
});

test("other errors keep the backend's message", () => {
  assert.equal(apiErrorMessage(403, { error: "Forbidden" }, null), "Forbidden");
  assert.equal(apiErrorMessage(500, {}, null), "HTTP error! status: 500");
});
