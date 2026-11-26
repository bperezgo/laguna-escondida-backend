import { planRequiredNotice } from "../plans.ts";
import { supportEmail } from "../support.ts";

export interface ErrorBodyLike {
  error?: string;
  message?: string;
  code?: string;
  required_plan?: string;
}

// The message an API error shows; a plan refusal reads as an upgrade notice wherever it surfaces.
export function apiErrorMessage(
  status: number,
  body: ErrorBodyLike,
  email: string | null = supportEmail()
): string {
  if (status === 403 && body.code === "plan_required") {
    return planRequiredNotice(body.required_plan, email).message;
  }
  return body.error || body.message || `HTTP error! status: ${status}`;
}
