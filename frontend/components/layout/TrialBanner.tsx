"use client";

import { usePermissions } from "@/lib/permissions";
import { useNow } from "@/lib/kitchen/useNow";
import { supportEmail, TRIAL_EXPIRED_MESSAGE } from "@/lib/support";
import { trialBanner } from "@/lib/trial";

export default function TrialBanner() {
  const { organization, isEdge, isLoading } = usePermissions();
  const now = useNow(60_000);
  const banner = isLoading ? null : trialBanner(organization, isEdge, now, supportEmail());
  if (!banner) return null;

  return (
    <div
      role="status"
      style={{
        padding: "0.5rem 1rem",
        textAlign: "center",
        fontSize: "0.875rem",
        fontWeight: 500,
        backgroundColor: "var(--color-warning-bg)",
        color: "var(--color-warning-text)",
      }}
    >
      {banner.message}
      {banner.mailto && (
        <>
          {" "}
          <a href={banner.mailto} style={{ color: "inherit", fontWeight: 700, textDecoration: "underline" }}>
            {banner.message === TRIAL_EXPIRED_MESSAGE ? "Escríbenos" : "Escríbenos para continuar"}
          </a>
        </>
      )}
    </div>
  );
}
