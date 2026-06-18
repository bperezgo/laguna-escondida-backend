"use client";

import { useState } from "react";
import type { Product, ProductType } from "@/types/product";
import {
  PRODUCT_TYPES,
  UNITS_OF_MEASURE,
  requiresPricing,
} from "@/types/product";
import { Button, Input, Select, Table, Badge } from "@/components/ui";
import type { BadgeTone } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface ProductListProps {
  products: Product[];
  onEdit: (product: Product) => void;
  onDelete: (id: string) => void;
  isLoading?: boolean;
}

const formatCurrency = (amount: number) => {
  return new Intl.NumberFormat("es-CO", {
    style: "currency",
    currency: "COP",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount);
};

const getProductTypeLabel = (product: Product) => {
  const type = PRODUCT_TYPES.find((t) => t.value === product.product_type);
  return type?.label || product.product_type;
};

const getUnitLabel = (product: Product) => {
  const unit = UNITS_OF_MEASURE.find(
    (u) => u.value === product.unit_of_measure
  );
  return unit?.label || product.unit_of_measure;
};

const getProductTypeTone = (productType: ProductType): BadgeTone => {
  switch (productType) {
    case "SELLABLE":
      return "success";
    case "INGREDIENT":
      return "warning";
    case "COMPOSITE":
      return "info";
    case "BOTH":
      return "neutral";
    default:
      return "neutral";
  }
};

export default function ProductList({
  products,
  onEdit,
  onDelete,
  isLoading = false,
}: ProductListProps) {
  const [searchTerm, setSearchTerm] = useState("");
  const [filterCategory, setFilterCategory] = useState("");

  // Get unique categories from products
  const categories = Array.from(
    new Set(products.map((p) => p.category))
  ).sort();

  // Filter products based on search term and category
  const filteredProducts = products.filter((product) => {
    const matchesSearch =
      product.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      product.category.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesCategory =
      !filterCategory || product.category === filterCategory;
    return matchesSearch && matchesCategory;
  });

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando productos...
        </p>
      </div>
    );
  }

  if (products.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron productos. ¡Crea tu primer producto!
        </p>
      </div>
    );
  }

  return (
    <div>
      <div
        style={{
          marginBottom: "2rem",
          padding: "1.5rem",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
          display: "flex",
          gap: "1rem",
          flexWrap: "wrap",
          alignItems: "center",
        }}
      >
        <div style={{ flex: 1, minWidth: 200 }}>
          <Input
            label="Buscar Productos"
            placeholder="Buscar por nombre o categoría..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>
        <div style={{ flex: 1, minWidth: 200 }}>
          <Select
            label="Filtrar por Categoría"
            value={filterCategory}
            onChange={(e) => setFilterCategory(e.target.value)}
          >
            <option value="">Todas las Categorías</option>
            {categories.map((category) => (
              <option key={category} value={category}>
                {category}
              </option>
            ))}
          </Select>
        </div>
      </div>

      <div
        style={{
          marginBottom: "1rem",
          color: "var(--color-text-secondary)",
          fontSize: "0.9rem",
        }}
      >
        Mostrando {filteredProducts.length} de {products.length} producto
        {products.length !== 1 ? "s" : ""}
      </div>

      {filteredProducts.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p
            style={{
              fontSize: "1.1rem",
              color: "var(--color-text-secondary)",
            }}
          >
            No hay productos que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Producto</th>
              <th>Tipo</th>
              <th>Categoría</th>
              <th>Unidad</th>
              <th data-numeric>Precio Total</th>
              <th style={{ textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {filteredProducts.map((product) => (
              <tr key={product.id}>
                <td>
                  <div style={{ fontWeight: 600 }}>{product.name}</div>
                  <div
                    style={{
                      fontFamily: "monospace",
                      fontSize: "0.8rem",
                      color: "var(--color-text-muted)",
                    }}
                  >
                    SKU: {product.sku}
                  </div>
                </td>
                <td>
                  <Badge
                    tone={getProductTypeTone(product.product_type)}
                    dot={false}
                  >
                    {getProductTypeLabel(product)}
                  </Badge>
                </td>
                <td>{product.category}</td>
                <td>{getUnitLabel(product)}</td>
                <td data-numeric style={{ fontWeight: 600 }}>
                  {requiresPricing(product.product_type) ? (
                    formatCurrency(
                      parseFloat(product.total_price_with_taxes || "0")
                    )
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
                    }}
                  >
                    <PermissionGate permission={PERMISSIONS.PRODUCTS_UPDATE}>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => onEdit(product)}
                      >
                        Editar
                      </Button>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.PRODUCTS_DELETE}>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => onDelete(product.id)}
                      >
                        Eliminar
                      </Button>
                    </PermissionGate>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
