/**
 * Permission system types
 */

import type { EdgeMode } from "@/types/edge";

export interface UserRole {
  id: number;
  name: string;
  created_at?: string;
}

/**
 * The organization the signed-in user works in (GET /auth/me). Optional
 * settings are null until the organization sets them; callers fall back to
 * today's Laguna Escondida defaults.
 */
export interface Organization {
  id: string;
  slug: string;
  name: string;
  /** Absent from a backend older than plans and trials. */
  status?: "trial" | "active" | "suspended";
  plan?: "starter" | "einvoicing" | "full";
  /** RFC 3339; set while status is trial. */
  trial_ends_at?: string | null;
  /** IANA time zone of the business day, e.g. America/Bogota. */
  timezone: string;
  /** ISO 3166-1 alpha-2, e.g. CO. */
  country: string;
  locale: string;
  /** ISO 4217, e.g. COP. */
  currency: string;
  legal_name: string | null;
  nit: string | null;
  address: string | null;
  phone: string | null;
  ticket_footer: string | null;
  ticket_legal_notice: string | null;
  display_name: string | null;
  logo_url: string | null;
  /** #RRGGBB */
  primary_color: string | null;
  /** #RRGGBB */
  secondary_color: string | null;
}

/**
 * What the subscription notice needs (GET /auth/me `billing`). Present only in the cloud and only
 * for a user with billing:manage.
 */
export interface BillingSummary {
  period: "monthly" | "annual";
  has_card: boolean;
  card_entry: boolean;
  cancelled: boolean;
  /** RFC 3339: the trial's end during the trial, the end of the paid period after. */
  due_date: string | null;
  /** Pesos; null when nothing will be charged. */
  next_charge_amount: number | null;
  payment_failed: boolean;
  /** RFC 3339: when the organization stops unless a charge is approved. */
  access_ends_at: string | null;
}

export interface AuthUser {
  id: string;
  username: string;
  name?: string;
  roles: UserRole[];
  permissions: string[];
  /** Absent only from a backend older than the multi-tenancy release. */
  organization?: Organization;
  /** Only in the cloud, only for a user who manages billing. */
  billing?: BillingSummary;
  /** Deployment mode injected by the Next.js /api/auth/me proxy. */
  deployment_mode?: EdgeMode;
}

export interface PermissionsContextValue {
  user: AuthUser | null;
  /** The signed-in user's organization; null while loading or signed out. */
  organization: Organization | null;
  /** The subscription summary of a user who manages billing; null for anyone else. */
  billing: BillingSummary | null;
  permissions: string[];
  isLoading: boolean;
  error: string | null;
  /** True when the current user has the "admin" role. */
  isAdmin: boolean;
  /** True when the frontend is served from the edge node (not cloud). */
  isEdge: boolean;
  hasPermission: (permission: string) => boolean;
  hasAnyPermission: (permissions: string[]) => boolean;
  hasAllPermissions: (permissions: string[]) => boolean;
  fetchPermissions: () => Promise<void>;
}
