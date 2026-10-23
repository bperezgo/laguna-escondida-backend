"use client";

import { useEffect, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { usePermissions } from "@/lib/permissions";
import { resetFormatSettings, setFormatSettings } from "@/lib/format";
import { brandingCss, organizationDisplayName } from "@/lib/organization";

/**
 * Wraps the signed-in screens (not /signin, which keeps the Laguna branding)
 * and dresses them in the organization's settings from /auth/me: number, date
 * and currency formatting, brand colors and the document title.
 *
 * Children render once the organization is known, so nothing formats with the
 * defaults first. A session the backend rejects (e.g. a token from before
 * multi-tenancy) goes back to /signin.
 */
export default function OrganizationBranding({
  children,
}: {
  children: ReactNode;
}) {
  const { user, organization, isLoading, error } = usePermissions();
  const pathname = usePathname();

  // Before the children render, so their first render already formats right.
  // OrganizationBranding never sees an organization on the server (/auth/me is
  // fetched in an effect), so this never touches another request's settings.
  if (organization) {
    setFormatSettings({
      locale: organization.locale,
      currency: organization.currency,
      timeZone: organization.timezone,
    });
  }

  useEffect(() => resetFormatSettings, []);

  // Rendered with the children (not added in an effect), so it is in place
  // before their effects run: the delivery-zone map reads the colors then.
  const css = organization ? brandingCss(organization) : null;

  const title = organization ? organizationDisplayName(organization) : null;
  useEffect(() => {
    if (title) document.title = title;
  }, [title, pathname]);

  const signedOut = !isLoading && !user && !error;
  useEffect(() => {
    if (signedOut) {
      window.location.href = `/signin?redirect=${encodeURIComponent(pathname)}`;
    }
  }, [signedOut, pathname]);

  // A failed /auth/me (error) or an older backend without an organization
  // still renders, with the defaults.
  const ready = organization !== null || (!isLoading && user !== null) || !!error;
  if (!ready) return null;
  return (
    <>
      {css && <style>{css}</style>}
      {children}
    </>
  );
}
