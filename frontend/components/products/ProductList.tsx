"use client";

import { useState } from "react";
import type { Product } from "@/types/product";
import ProductCard from "./ProductCard";

interface ProductListProps {
  products: Product[];
  onEdit: (product: Product) => void;
  onDelete: (id: string) => void;
  isLoading?: boolean;
}

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
        <div style={{ flex: "1", minWidth: "200px" }}>
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: "500",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Buscar Productos
          </label>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Buscar por nombre o categoría..."
            style={{
              width: "100%",
              padding: "0.75rem",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              fontSize: "1rem",
              boxSizing: "border-box",
              backgroundColor: "var(--color-bg)",
              color: "var(--color-text-primary)",
            }}
          />
        </div>
        <div style={{ flex: "1", minWidth: "200px" }}>
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: "500",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Filtrar por Categoría
          </label>
          <select
            value={filterCategory}
            onChange={(e) => setFilterCategory(e.target.value)}
            style={{
              width: "100%",
              padding: "0.75rem",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              fontSize: "1rem",
              boxSizing: "border-box",
              backgroundColor: "var(--color-bg)",
              color: "var(--color-text-primary)",
            }}
          >
            <option value="">Todas las Categorías</option>
            {categories.map((category) => (
              <option key={category} value={category}>
                {category}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div style={{ marginBottom: "1rem", color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
        Mostrando {filteredProducts.length} de {products.length} producto
        {products.length !== 1 ? "s" : ""}
      </div>

      {filteredProducts.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay productos que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <div>
          {filteredProducts.map((product) => (
            <ProductCard
              key={product.id}
              product={product}
              onEdit={onEdit}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
