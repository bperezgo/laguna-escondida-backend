import type { Role } from "../types/user.ts";
import type { Invitation, InvitationLookup, IssuedInvitation } from "../types/invitation.ts";

export const INVITABLE_ROLE_NAMES = ["admin", "manager", "accountant", "courier"];
export const MIN_PASSWORD_LENGTH = 8;
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

// Admin is offered only to admins; agent and floor roles are never invited.
export function invitableRoles(roles: Role[], inviterIsAdmin: boolean): Role[] {
  return roles.filter(
    (role) => INVITABLE_ROLE_NAMES.includes(role.name) && (inviterIsAdmin || role.name !== "admin")
  );
}

export interface InviteForm {
  email: string;
  name: string;
  roleIds: number[];
}

export type InviteErrors = Partial<Record<"email" | "name" | "roles", string>>;

export function validateInvite({ email, name, roleIds }: InviteForm): InviteErrors {
  const errors: InviteErrors = {};
  if (!email.trim()) errors.email = "Escribe el correo de la persona.";
  else if (!EMAIL_PATTERN.test(email.trim())) errors.email = "Ingresa un correo válido.";
  const trimmed = name.trim();
  if (!trimmed) errors.name = "Escribe el nombre de la persona.";
  else if (Array.from(trimmed).length > 255) errors.name = "El nombre admite hasta 255 caracteres.";
  if (roleIds.length === 0) errors.roles = "Selecciona al menos un rol.";
  return errors;
}

export function inviteBody({ email, name, roleIds }: InviteForm) {
  return { email: email.trim(), name: name.trim(), role_ids: roleIds };
}

const INVITE_ERRORS: Record<string, string> = {
  "invitations are for admin, manager, accountant or courier roles only":
    "Solo se invita a Administrador, Gerente, Contador o Repartidor.",
  "Only an admin can invite an admin": "Solo un administrador puede invitar a otro administrador.",
  "Email is already in use": "Ese correo ya lo usa otro usuario.",
  "This email already has a pending invitation; get a new link for it instead":
    "Ese correo ya tiene una invitación pendiente: genera un enlace nuevo para ella.",
};

export function inviteErrorMessage(status: number, message: string): string {
  if (INVITE_ERRORS[message]) return INVITE_ERRORS[message];
  if (status === 404) return "La invitación ya no está pendiente.";
  return message || "Error al administrar las invitaciones.";
}

// After create or new link: the invitation goes to the top of the pending list, replacing itself.
export function invitationIssued(pending: Invitation[], issued: IssuedInvitation): Invitation[] {
  return [issued.invitation, ...pending.filter((i) => i.id !== issued.invitation.id)];
}

export function invitationRevoked(pending: Invitation[], id: string): Invitation[] {
  return pending.filter((i) => i.id !== id);
}

export function invitationExpired(invitation: { expires_at: string }, now: number): boolean {
  const end = Date.parse(invitation.expires_at);
  return !Number.isNaN(end) && end <= now;
}

export type LookupState =
  | { kind: "loaded"; invitation: InvitationLookup }
  | { kind: "invalid" }
  | { kind: "error"; message: string };

export function lookupState(status: number, body: Record<string, unknown>): LookupState {
  if (status === 200 && typeof body.organization_name === "string") {
    return { kind: "loaded", invitation: body as unknown as InvitationLookup };
  }
  if (body.code === "invalid_or_expired_link" || status === 400 || status === 404) return { kind: "invalid" };
  if (status === 429) return { kind: "error", message: "Demasiados intentos. Espera un momento e intenta de nuevo." };
  return { kind: "error", message: "No pudimos cargar la invitación. Intenta de nuevo." };
}

export function validateAcceptPassword(password: string, confirm: string): string {
  if (password.length < MIN_PASSWORD_LENGTH) return `La contraseña debe tener al menos ${MIN_PASSWORD_LENGTH} caracteres.`;
  if (password !== confirm) return "Las contraseñas no coinciden.";
  return "";
}

export type AcceptFailure =
  | { kind: "invalid" }
  | { kind: "organization_inactive" }
  | { kind: "email_taken" }
  | { kind: "message"; message: string };

export function acceptFailure(
  status: number,
  body: { error?: string; code?: string },
  invitedEmail: string
): AcceptFailure {
  if (body.code === "invalid_or_expired_link") return { kind: "invalid" };
  if (body.code === "organization_inactive" || status === 403) return { kind: "organization_inactive" };
  if (status === 409) return { kind: "email_taken" };
  if (body.error === "The Google account's email is not the invited one") {
    return { kind: "message", message: `Esa cuenta de Google no es la invitada. Usa la cuenta de ${invitedEmail}.` };
  }
  if (status === 429) return { kind: "message", message: "Demasiados intentos. Espera un momento e intenta de nuevo." };
  if (status === 401) return { kind: "message", message: "No pudimos verificar tu cuenta de Google. Intenta de nuevo." };
  if (status === 400) {
    return { kind: "message", message: `Revisa la contraseña: necesita al menos ${MIN_PASSWORD_LENGTH} caracteres.` };
  }
  return { kind: "message", message: "No pudimos aceptar la invitación. Intenta de nuevo." };
}
