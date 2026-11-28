"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { productsApi } from "@/lib/api/products";
import { gridAccess } from "@/lib/catalogGrid/access";
import type { ExistingProduct } from "@/lib/catalogGrid/state";
import CatalogGrid from "@/components/catalog-grid/CatalogGrid";

const page = {
  minHeight: "100vh",
  backgroundColor: "var(--color-bg)",
  padding: "2rem",
} as const;

function Notice({ children }: { children: ReactNode }) {
  return (
    <div style={page}>
      <div style={{ maxWidth: "720px", margin: "0 auto", color: "var(--color-text-secondary)" }}>
        <h1 style={{ color: "var(--color-text-primary)", fontSize: "1.75rem" }}>Catálogo inicial</h1>
        <p>{children}</p>
        <Link href="/products" style={{ color: "var(--color-primary)" }}>
          Volver a productos
        </Link>
      </div>
    </div>
  );
}

export default function CatalogGridPage() {
  const { isLoading, isEdge, hasPermission, user, organization } = usePermissions();
  const access = gridAccess({ isLoading, isEdge, canImport: hasPermission(PERMISSIONS.CATALOG_IMPORT) });

  if (access === "loading") return <Notice>Cargando…</Notice>;
  if (access === "edge") {
    return <Notice>El catálogo inicial solo está disponible en la nube. Ábrelo desde la versión web.</Notice>;
  }
  if (access === "forbidden" || !user || !organization) {
    return <Notice>Esta página no está disponible para tu rol.</Notice>;
  }
  return <CatalogGridLoader organizationId={organization.id} userId={user.id} />;
}

function CatalogGridLoader({ organizationId, userId }: { organizationId: string; userId: string }) {
  const [existing, setExisting] = useState<ExistingProduct[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    productsApi
      .getAll()
      .then((products) => {
        if (cancelled) return;
        setExisting(
          products.map((p) => ({
            id: p.id,
            name: p.name,
            sku: p.sku,
            product_type: p.product_type,
            unit_of_measure: p.unit_of_measure,
          })),
        );
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "No se pudieron cargar los productos");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <Notice>{error}</Notice>;
  if (!existing) return <Notice>Cargando productos…</Notice>;
  return (
    <div style={page}>
      <CatalogGrid organizationId={organizationId} userId={userId} existing={existing} />
    </div>
  );
}
