"use client";

import { ReactNode } from "react";
import { PermissionsProvider } from "@/lib/permissions";

interface ProvidersProps {
  children: ReactNode;
}

export function Providers({ children }: ProvidersProps) {
  return <PermissionsProvider>{children}</PermissionsProvider>;
}
