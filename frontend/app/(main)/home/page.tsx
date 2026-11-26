"use client";

import { useEffect, useMemo } from "react";
import { useRouter } from "next/navigation";
import Dashboard from "@/components/dashboard/Dashboard";
import NoAccess from "@/components/dashboard/NoAccess";
import { firstScreen, HOME_ENTRY } from "@/lib/navigation/shell";
import { usePermissions } from "@/lib/permissions";

export default function HomePage() {
  const router = useRouter();
  const { organization, isLoading, isAdmin, isEdge, hasPermission } = usePermissions();
  const target = useMemo(
    () => (isLoading ? undefined : firstScreen({ hasPermission, plan: organization?.plan, isEdge, isAdmin })),
    [isLoading, hasPermission, organization?.plan, isEdge, isAdmin]
  );
  const elsewhere = typeof target === "string" && target !== HOME_ENTRY.href ? target : null;

  useEffect(() => {
    if (elsewhere) router.replace(elsewhere);
  }, [elsewhere, router]);

  if (target === undefined || elsewhere) return null;
  return target === null ? <NoAccess /> : <Dashboard />;
}
