"use client";

import { subscriptionNotice } from "@/lib/billing/notice";
import { useNow } from "@/lib/kitchen/useNow";
import { usePermissions } from "@/lib/permissions";
import { supportEmail } from "@/lib/support";

// The one notice at the top of every page: the trial's days left, a failed charge, or a
// cancelled subscription's last day. Nothing for a paid organization, and nothing on a box.
export default function SubscriptionNotice() {
  const { organization, billing, isEdge, isLoading } = usePermissions();
  const now = useNow(60_000);
  const notice = isLoading ? null : subscriptionNotice({ organization, billing, isEdge, now, supportEmail: supportEmail() });
  if (!notice) return null;

  const danger = notice.tone === "danger";
  return (
    <div
      role="status"
      style={{
        padding: "0.5rem 1rem",
        textAlign: "center",
        fontSize: "0.875rem",
        fontWeight: 500,
        backgroundColor: danger ? "var(--color-danger-light)" : "var(--color-warning-bg)",
        color: danger ? "var(--color-danger)" : "var(--color-warning-text)",
      }}
    >
      {notice.message}
      {notice.link && (
        <>
          {" "}
          <a href={notice.link.href} style={{ color: "inherit", fontWeight: 700, textDecoration: "underline" }}>
            {notice.link.label}
          </a>
        </>
      )}
    </div>
  );
}
