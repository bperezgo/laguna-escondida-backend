import { test } from "node:test";
import assert from "node:assert/strict";
import { boxPasswordReducer, codeExpired, findBoxUser, formatCountdown, secondsLeft } from "./installBox.ts";
import type { UserWithRoles } from "../../types/user.ts";

const issued = Date.parse("2026-11-25T15:00:00Z");
const expiresAt = "2026-11-25T15:30:00Z";

test("the countdown runs from 30 minutes to zero", () => {
  assert.equal(formatCountdown(secondsLeft(expiresAt, issued)), "30:00");
  assert.equal(formatCountdown(secondsLeft(expiresAt, issued + 1000)), "29:59");
  assert.equal(formatCountdown(secondsLeft(expiresAt, issued + 29 * 60_000 + 55_500)), "0:05");
  assert.equal(formatCountdown(secondsLeft(expiresAt, issued + 31 * 60_000)), "0:00");
});

test("the code expires at expires_at", () => {
  assert.equal(codeExpired(expiresAt, issued + 29 * 60_000), false);
  assert.equal(codeExpired(expiresAt, Date.parse(expiresAt)), true);
  assert.equal(codeExpired("garbage", issued), true);
});

const user = (id: string, email: string | null, roles: string[], created_at: string, active = true): UserWithRoles => ({
  user: { id, username: id, name: id, email, active, created_at, updated_at: created_at },
  roles: roles.map((name, i) => ({ id: i, name })),
});

test("the box user is the manager without an email", () => {
  const users = [
    user("admin", "rosa@example.com", ["admin"], "2026-11-01T00:00:00Z"),
    user("mesero", null, ["server"], "2026-11-01T00:00:00Z"),
    user("gerente", "g@example.com", ["manager"], "2026-11-01T00:00:00Z"),
    user("caja2", null, ["manager"], "2026-11-03T00:00:00Z"),
    user("caja", null, ["manager"], "2026-11-02T00:00:00Z"),
    user("viejo", null, ["manager"], "2026-10-01T00:00:00Z", false),
  ];
  assert.equal(findBoxUser(users)?.user.id, "caja");
  assert.equal(findBoxUser(users.slice(0, 3)), null);
});

test("the generated password is shown once and cleared when hidden or on leaving", () => {
  const shown = boxPasswordReducer(null, { type: "generated", shown: { username: "rosa.perez", password: "K7F29QXMH3RD4T" } }); // gitleaks:allow
  assert.deepEqual(shown, { username: "rosa.perez", password: "K7F29QXMH3RD4T" }); // gitleaks:allow
  assert.equal(boxPasswordReducer(shown, { type: "hidden" }), null);
  assert.equal(boxPasswordReducer(shown, { type: "left" }), null);
});
