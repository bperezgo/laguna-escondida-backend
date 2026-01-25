"use client";

import type { Product } from "@/types/product";

interface ProductCardProps {
  product: Product;
  onEdit: (product: Product) => void;
  onDelete: (id: string) => void;
}

export default function ProductCard({
  product,
  onEdit,
  onDelete,
}: ProductCardProps) {
  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
    }).format(amount);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const totalPrice = product.total_price_with_taxes;

  return (
    <div
      style={{
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        padding: "1.5rem",
        marginBottom: "1rem",
        backgroundColor: "var(--color-surface)",
        boxShadow: "var(--shadow-sm)",
        transition: "box-shadow var(--transition-normal)",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-md)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-sm)";
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          marginBottom: "1rem",
        }}
      >
        <div style={{ flex: 1 }}>
          <h3
            style={{
              margin: "0 0 0.25rem 0",
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {product.name}
          </h3>
          <p
            style={{
              margin: "0 0 0.75rem 0",
              fontFamily: "monospace",
              fontSize: "0.85rem",
              color: "var(--color-text-muted)",
              backgroundColor: "var(--color-bg)",
              padding: "0.25rem 0.5rem",
              borderRadius: "var(--radius-sm)",
              display: "inline-block",
            }}
          >
            SKU: {product.sku}
          </p>
          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "var(--color-text-secondary)",
              fontSize: "0.9rem",
            }}
          >
            <strong>Categoría:</strong> {product.category}
          </p>
          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "var(--color-text-secondary)",
              fontSize: "0.9rem",
            }}
          >
            <strong>Versión:</strong> {product.version}
          </p>
        </div>
        <div style={{ display: "flex", gap: "0.5rem" }}>
          <button
            onClick={() => onEdit(product)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "var(--color-primary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "background-color var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-primary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
            }}
          >
            Editar
          </button>
          <button
            onClick={() => onDelete(product.id)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "var(--color-danger)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "background-color var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-danger-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-danger)";
            }}
          >
            Eliminar
          </button>
        </div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: "1rem",
          padding: "1rem",
          backgroundColor: "var(--color-bg)",
          borderRadius: "var(--radius-sm)",
          border: "1px solid var(--color-border)",
        }}
      >
        <div>
          <p
            style={{
              margin: "0 0 0.25rem 0",
              color: "var(--color-text-muted)",
              fontSize: "0.85rem",
            }}
          >
            Precio Base
          </p>
          <p
            style={{
              margin: 0,
              fontSize: "1.1rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {formatCurrency(parseFloat(product.unit_price))}
          </p>
        </div>
        <div>
          <p
            style={{
              margin: "0 0 0.25rem 0",
              color: "var(--color-text-muted)",
              fontSize: "0.85rem",
            }}
          >
            VAT ({parseFloat(product.vat) * 100}%)
          </p>
          <p
            style={{
              margin: 0,
              fontSize: "1.1rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {formatCurrency(
              parseFloat(product.unit_price) * parseFloat(product.vat)
            )}
          </p>
        </div>
        <div>
          <p
            style={{
              margin: "0 0 0.25rem 0",
              color: "var(--color-text-muted)",
              fontSize: "0.85rem",
            }}
          >
            ICO ({parseFloat(product.ico) * 100}%)
          </p>
          <p
            style={{
              margin: 0,
              fontSize: "1.1rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {formatCurrency(
              parseFloat(product.unit_price) * parseFloat(product.ico)
            )}
          </p>
        </div>
        <div
          style={{
            gridColumn: "1 / -1",
            borderTop: "1px solid var(--color-border)",
            paddingTop: "0.5rem",
          }}
        >
          <p
            style={{
              margin: "0 0 0.25rem 0",
              color: "var(--color-text-muted)",
              fontSize: "0.85rem",
            }}
          >
            Precio Total
          </p>
          <p
            style={{
              margin: 0,
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-success)",
            }}
          >
            {formatCurrency(parseFloat(totalPrice))}
          </p>
        </div>
      </div>

      <div
        style={{
          marginTop: "1rem",
          paddingTop: "1rem",
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <p
          style={{
            margin: "0.25rem 0",
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Creado: {formatDate(product.created_at)}
        </p>
        <p
          style={{
            margin: "0.25rem 0",
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Actualizado: {formatDate(product.updated_at)}
        </p>
      </div>
    </div>
  );
}
