import { test } from "node:test";
import assert from "node:assert/strict";
import {
  acceptFailure,
  invitableRoles,
  invitationExpired,
  invitationIssued,
  invitationRevoked,
  inviteBody,
  inviteErrorMessage,
  lookupState,
  validateAcceptPassword,
  validateInvite,
} from "./invitations.ts";
import type { Invitation } from "../types/invitation.ts";

const roles = [
  { id: 1, name: "server" },
  { id: 2, name: "admin" },
  { id: 3, name: "manager" },
  { id: 4, name: "cooker" },
  { id: 5, name: "accountant" },
  { id: 6, name: "agent" },
  { id: 7, name: "courier" },
];

const invitation = (id: string): Invitation => ({
  id,
  email: `${id}@example.com`,
  name: id,
  role_ids: [5],
  roles: [{ id: 5, name: "accountant" }],
  invited_by: "u1",
  invited_by_name: "",
  expires_at: "2026-12-02T15:00:00Z",
  created_at: "2026-11-25T15:00:00Z",
});

test("an admin may invite admins, managers, accountants and couriers", () => {
  assert.deepEqual(invitableRoles(roles, true).map((r) => r.name), ["admin", "manager", "accountant", "courier"]);
});

test("a non-admin is not offered the admin role", () => {
  assert.deepEqual(invitableRoles(roles, false).map((r) => r.name), ["manager", "accountant", "courier"]);
});

test("the invite form needs an email, a name and a role", () => {
  assert.deepEqual(validateInvite({ email: "", name: " ", roleIds: [] }), {
    email: "Escribe el correo de la persona.",
    name: "Escribe el nombre de la persona.",
    roles: "Selecciona al menos un rol.",
  });
  assert.equal(validateInvite({ email: "no-at", name: "Marta", roleIds: [5] }).email, "Ingresa un correo válido.");
  assert.deepEqual(validateInvite({ email: "m@example.com", name: "Marta", roleIds: [5] }), {});
  assert.deepEqual(inviteBody({ email: " m@example.com ", name: " Marta ", roleIds: [5] }), {
    email: "m@example.com",
    name: "Marta",
    role_ids: [5],
  });
});

test("invite errors read in Spanish", () => {
  assert.match(inviteErrorMessage(409, "Email is already in use"), /ya lo usa/);
  assert.match(
    inviteErrorMessage(409, "This email already has a pending invitation; get a new link for it instead"),
    /enlace nuevo/
  );
  assert.match(inviteErrorMessage(403, "Only an admin can invite an admin"), /Solo un administrador/);
  assert.equal(inviteErrorMessage(404, "Not found"), "La invitación ya no está pendiente.");
});

test("create and new link put the invitation on top, once", () => {
  const pending = [invitation("a"), invitation("b")];
  const created = invitationIssued(pending, { invitation: invitation("c"), link: "https://x/invitacion/t" });
  assert.deepEqual(created.map((i) => i.id), ["c", "a", "b"]);
  const relinked = invitationIssued(created, { invitation: invitation("b"), link: "https://x/invitacion/t2" });
  assert.deepEqual(relinked.map((i) => i.id), ["b", "c", "a"]);
});

test("revoke removes the invitation from the pending list", () => {
  assert.deepEqual(invitationRevoked([invitation("a"), invitation("b")], "a").map((i) => i.id), ["b"]);
});

test("an invitation past its expiry is marked expired", () => {
  assert.equal(invitationExpired(invitation("a"), Date.parse("2026-12-02T15:00:01Z")), true);
  assert.equal(invitationExpired(invitation("a"), Date.parse("2026-12-01T15:00:00Z")), false);
});

test("the acceptance page shows a valid invitation", () => {
  const state = lookupState(200, {
    organization_name: "Doña Rosa",
    email: "m@example.com",
    name: "Marta",
    roles: [],
    expires_at: "2026-12-02T15:00:00Z",
  });
  assert.equal(state.kind, "loaded");
});

test("an invalid or expired link shows the invalid state", () => {
  assert.deepEqual(lookupState(400, { error: "Invalid or expired link", code: "invalid_or_expired_link" }), { kind: "invalid" });
  assert.equal(lookupState(429, {}).kind, "error");
  assert.equal(lookupState(500, {}).kind, "error");
});

test("the password needs 8 characters and a matching confirmation", () => {
  assert.match(validateAcceptPassword("short", "short"), /al menos 8/);
  assert.equal(validateAcceptPassword("una-clave-segura", "otra"), "Las contraseñas no coinciden.");
  assert.equal(validateAcceptPassword("una-clave-segura", "una-clave-segura"), "");
});

test("each acceptance failure maps to its state", () => {
  assert.deepEqual(acceptFailure(400, { code: "invalid_or_expired_link" }, "m@example.com"), { kind: "invalid" });
  assert.deepEqual(acceptFailure(403, { code: "organization_inactive" }, "m@example.com"), { kind: "organization_inactive" });
  assert.deepEqual(acceptFailure(409, { error: "Email is already in use" }, "m@example.com"), { kind: "email_taken" });
  const mismatch = acceptFailure(400, { error: "The Google account's email is not the invited one" }, "m@example.com");
  assert.equal(mismatch.kind, "message");
  assert.match(mismatch.kind === "message" ? mismatch.message : "", /m@example\.com/);
});
