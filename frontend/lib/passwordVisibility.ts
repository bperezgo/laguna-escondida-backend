export type PasswordVisibilityAction = "toggled" | "submitted" | "reset";

export const SHOW_PASSWORD_LABEL = "Mostrar contraseña";
export const HIDE_PASSWORD_LABEL = "Ocultar contraseña";

export function toggleLabel(shown: boolean): string {
  return shown ? HIDE_PASSWORD_LABEL : SHOW_PASSWORD_LABEL;
}

export function inputType(shown: boolean): "text" | "password" {
  return shown ? "text" : "password";
}

export function passwordVisibilityReducer(shown: boolean, action: PasswordVisibilityAction): boolean {
  return action === "toggled" ? !shown : false;
}
