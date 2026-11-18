"use client";

import { useState } from "react";
import { productsApi } from "@/lib/api/products";
import type {
  Product,
  CreateProductRequest,
  UpdateProductRequest,
} from "@/types/product";
import ProductList from "@/components/products/ProductList";
import ProductForm from "@/components/products/ProductForm";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface ProductsPageClientProps {
  initialProducts: Product[];
}

export default function ProductsPageClient({
  initialProducts,
}: ProductsPageClientProps) {
  const [products, setProducts] = useState<Product[]>(initialProducts);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [editingProduct, setEditingProduct] = useState<Product | null>(null);
  const [formLoading, setFormLoading] = useState<boolean>(false);

  const loadProducts = async () => {
    try {
      setLoading(true);
      setError("");
      const fetchedProducts = await productsApi.getAll();
      setProducts(fetchedProducts);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to load products";
      setError(errorMessage);
      console.error("Error loading products:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingProduct(null);
    setShowForm(true);
  };

  const handleEdit = (product: Product) => {
    setEditingProduct(product);
    setShowForm(true);
  };

  const handleFormSubmit = async (
    data: CreateProductRequest | UpdateProductRequest
  ) => {
    try {
      setFormLoading(true);
      setError("");

      if (editingProduct) {
        // Update existing product
        await productsApi.update(
          editingProduct.id,
          data as UpdateProductRequest
        );
      } else {
        // Create new product
        await productsApi.create(data as CreateProductRequest);
      }

      // Reload products list
      await loadProducts();
      setShowForm(false);
      setEditingProduct(null);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to save product";
      setError(errorMessage);
      console.error("Error saving product:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("¿Estás seguro de que quieres eliminar este producto?")) {
      return;
    }

    try {
      setError("");
      await productsApi.delete(id);
      // Reload products list
      await loadProducts();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to delete product";
      setError(errorMessage);
      console.error("Error deleting product:", err);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setEditingProduct(null);
    setError("");
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem",
      }}
    >
      <div
        style={{
          maxWidth: "1200px",
          margin: "0 auto",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "2rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div>
            <h1
              style={{
                margin: 0,
                fontSize: "2rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Productos
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Administra los productos de tu restaurante
            </p>
          </div>
          <PermissionGate permission={PERMISSIONS.PRODUCTS_CREATE}>
            <button
              onClick={handleCreate}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-success)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                boxShadow: "var(--shadow-sm)",
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-success-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-success)";
              }}
            >
              + Crear Producto
            </button>
          </PermissionGate>
        </div>

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Error:</strong> {error}
          </div>
        )}

        {/* Products List */}
        <ProductList
          products={products}
          onEdit={handleEdit}
          onDelete={handleDelete}
          isLoading={loading}
        />

        {/* Product Form Modal */}
        {showForm && (
          <ProductForm
            product={editingProduct}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}
      </div>
    </div>
  );
}
