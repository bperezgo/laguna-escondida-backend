"use client";

import { useState } from "react";
import type { Supplier } from "@/types/supplier";
import { Button, Input, Table } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface SupplierListProps {
  suppliers: Supplier[];
  onEdit: (supplier: Supplier) => void;
  onDelete: (id: string) => void;
  onViewCatalog: (supplier: Supplier) => void;
  isLoading?: boolean;
}

export default function SupplierList({
  suppliers,
  onEdit,
  onDelete,
  onViewCatalog,
  isLoading = false,
}: SupplierListProps) {
  const [searchTerm, setSearchTerm] = useState("");

  // Filter suppliers based on search term
  const filteredSuppliers = suppliers.filter((supplier) => {
    const search = searchTerm.toLowerCase();
    return (
      supplier.name.toLowerCase().includes(search) ||
      supplier.contact_name?.toLowerCase().includes(search) ||
      supplier.email?.toLowerCase().includes(search) ||
      supplier.phone?.includes(search)
    );
  });

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando proveedores...
        </p>
      </div>
    );
  }

  if (suppliers.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron proveedores. ¡Crea tu primer proveedor!
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Search */}
      <div style={{ maxWidth: 400, marginBottom: "1.5rem" }}>
        <Input
          label="Buscar Proveedores"
          placeholder="Buscar por nombre, contacto, email o teléfono..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
      </div>

      {/* Results count */}
      <div
        style={{
          marginBottom: "1rem",
          color: "var(--color-text-secondary)",
          fontSize: "0.9rem",
        }}
      >
        Mostrando {filteredSuppliers.length} de {suppliers.length} proveedor
        {suppliers.length !== 1 ? "es" : ""}
      </div>

      {/* Supplier Table */}
      {filteredSuppliers.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay proveedores que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Identificación</th>
              <th>Contacto</th>
              <th>Teléfono</th>
              <th>Email</th>
              <th style={{ textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {filteredSuppliers.map((supplier) => {
              const hasIdentification =
                supplier.identification_type || supplier.identification_number;
              const identification = [
                supplier.identification_type,
                supplier.identification_number,
              ]
                .filter(Boolean)
                .join(" - ");

              return (
                <tr key={supplier.id}>
                  <td style={{ fontWeight: 600, color: "var(--color-text-primary)" }}>
                    {supplier.name}
                  </td>
                  <td>
                    {hasIdentification ? (
                      identification
                    ) : (
                      <span style={{ color: "var(--color-text-muted)" }}>—</span>
                    )}
                  </td>
                  <td>
                    {supplier.contact_name || (
                      <span style={{ color: "var(--color-text-muted)" }}>—</span>
                    )}
                  </td>
                  <td>
                    {supplier.phone || (
                      <span style={{ color: "var(--color-text-muted)" }}>—</span>
                    )}
                  </td>
                  <td>
                    {supplier.email ? (
                      <a
                        href={`mailto:${supplier.email}`}
                        style={{ color: "var(--color-primary)" }}
                      >
                        {supplier.email}
                      </a>
                    ) : (
                      <span style={{ color: "var(--color-text-muted)" }}>—</span>
                    )}
                  </td>
                  <td style={{ textAlign: "right" }}>
                    <div
                      style={{
                        display: "flex",
                        gap: "0.5rem",
                        justifyContent: "flex-end",
                        flexWrap: "wrap",
                      }}
                    >
                      <PermissionGate permission={PERMISSIONS.SUPPLIER_CATALOG_READ}>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => onViewCatalog(supplier)}
                        >
                          Catálogo
                        </Button>
                      </PermissionGate>
                      <PermissionGate permission={PERMISSIONS.SUPPLIERS_UPDATE}>
                        <Button
                          variant="primary"
                          size="sm"
                          onClick={() => onEdit(supplier)}
                        >
                          Editar
                        </Button>
                      </PermissionGate>
                      <PermissionGate permission={PERMISSIONS.SUPPLIERS_DELETE}>
                        <Button
                          variant="danger"
                          size="sm"
                          onClick={() => onDelete(supplier.id)}
                        >
                          Eliminar
                        </Button>
                      </PermissionGate>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </Table>
      )}
    </div>
  );
}
