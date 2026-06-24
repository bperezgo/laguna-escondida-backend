"use client";

import { ReactNode } from "react";
import { PermissionsProvider } from "@/lib/permissions";
import { EdgeStatusProvider } from "@/lib/edge/context";

interface ProvidersProps {
  children: ReactNode;
}

export function Providers({ children }: ProvidersProps) {
  return (
    <PermissionsProvider>
      <EdgeStatusProvider>{children}</EdgeStatusProvider>
    </PermissionsProvider>
  );
}
