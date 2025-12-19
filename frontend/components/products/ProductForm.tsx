"use client";

import { useState, useEffect } from "react";
import type {
  Product,
  CreateProductRequest,
  UpdateProductRequest,
} from "@/types/product";

interface ProductFormProps {
  product?: Product | null;
  onSubmit: (
    data: CreateProductRequest | UpdateProductRequest
  ) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export default function ProductForm({
  product,
  onSubmit,
  onCancel,
  isLoading = false,
}: ProductFormProps) {
  const [formData, setFormData] = useState({
    name: product?.name || "",
    category: product?.category || "",
    vat: product?.vat?.toString() || "",
    ico: product?.ico?.toString() || "",
    description: product?.description || "",
    brand: product?.brand || "",
    model: product?.model || "",
    sku: product?.sku || "",
    total_price_with_taxes: product?.total_price_with_taxes?.toString() || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (product) {
      setFormData({
        name: product.name || "",
        category: product.category || "",
        vat: product.vat?.toString() || "",
        ico: product.ico?.toString() || "",
        description: product.description || "",
        brand: product.brand || "",
        model: product.model || "",
        sku: product.sku || "",
        total_price_with_taxes:
          product.total_price_with_taxes?.toString() || "",
      });
    }
  }, [product]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Name: required, min=1, max=255
    if (!formData.name.trim()) {
      newErrors.name = "El nombre del producto es requerido";
    } else if (formData.name.length > 255) {
      newErrors.name =
        "El nombre del producto debe tener 255 caracteres o menos";
    }

    // Category: required, min=1, max=100
    if (!formData.category.trim()) {
      newErrors.category = "La categoría es requerida";
    } else if (formData.category.length > 100) {
      newErrors.category = "La categoría debe tener 100 caracteres o menos";
    }

    // VAT: required, gte=0 (as string)
    if (!formData.vat.trim()) {
      newErrors.vat = "El IVA es requerido";
    } else {
      const vat = parseFloat(formData.vat);
      if (isNaN(vat) || vat < 0) {
        newErrors.vat = "El IVA debe ser un número válido mayor o igual a 0";
      }
    }

    // ICO: required, gte=0 (as string)
    if (!formData.ico.trim()) {
      newErrors.ico = "El ICO es requerido";
    } else {
      const ico = parseFloat(formData.ico);
      if (isNaN(ico) || ico < 0) {
        newErrors.ico = "El ICO debe ser un número válido mayor o igual a 0";
      }
    }

    // SKU: required, min=1, max=255
    if (!formData.sku.trim()) {
      newErrors.sku = "El SKU es requerido";
    } else if (formData.sku.length > 255) {
      newErrors.sku = "El SKU debe tener 255 caracteres o menos";
    }

    // TotalPriceWithTaxes: required, gt=0 (as string)
    if (!formData.total_price_with_taxes.trim()) {
      newErrors.total_price_with_taxes =
        "El precio total con impuestos es requerido";
    } else {
      const totalPriceWithTaxes = parseFloat(formData.total_price_with_taxes);
      if (isNaN(totalPriceWithTaxes) || totalPriceWithTaxes <= 0) {
        newErrors.total_price_with_taxes =
          "El precio total con impuestos debe ser un número válido mayor a 0";
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    // Build submit data
    if (product) {
      // Update request - all fields are required
      const submitData: UpdateProductRequest = {
        name: formData.name.trim(),
        category: formData.category.trim(),
        vat: formData.vat.trim(),
        ico: formData.ico.trim(),
        taxes_format: "percentage",
        sku: formData.sku.trim(),
        total_price_with_taxes: formData.total_price_with_taxes.trim(),
      };

      // Add optional fields only if they have values
      if (formData.description.trim()) {
        submitData.description = formData.description.trim();
      }
      if (formData.brand.trim()) {
        submitData.brand = formData.brand.trim();
      }
      if (formData.model.trim()) {
        submitData.model = formData.model.trim();
      }

      await onSubmit(submitData);
    } else {
      // Create request - all fields are required
      const submitData: CreateProductRequest = {
        name: formData.name.trim(),
        category: formData.category.trim(),
        vat: formData.vat.trim(),
        ico: formData.ico.trim(),
        taxes_format: "percentage",
        sku: formData.sku.trim(),
        total_price_with_taxes: formData.total_price_with_taxes.trim(),
      };

      // Add optional fields only if they have values
      if (formData.description.trim()) {
        submitData.description = formData.description.trim();
      }
      if (formData.brand.trim()) {
        submitData.brand = formData.brand.trim();
      }
      if (formData.model.trim()) {
        submitData.model = formData.model.trim();
      }

      await onSubmit(submitData);
    }
  };

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          padding: "2rem",
          maxWidth: "500px",
          width: "100%",
          maxHeight: "90vh",
          overflowY: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
      >
        <h2
          style={{
            marginTop: 0,
            marginBottom: "1.5rem",
            fontSize: "1.5rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          {product ? "Editar Producto" : "Crear Nuevo Producto"}
        </h2>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Nombre del Producto *
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => handleChange("name", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.name ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa el nombre del producto"
              maxLength={255}
            />
            {errors.name && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.name}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Categoría *
            </label>
            <input
              type="text"
              value={formData.category}
              onChange={(e) => handleChange("category", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.category ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa la categoría"
              maxLength={100}
            />
            {errors.category && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.category}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              SKU *
            </label>
            <input
              type="text"
              value={formData.sku}
              onChange={(e) => handleChange("sku", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.sku ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa el SKU"
              maxLength={255}
            />
            {errors.sku && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.sku}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Precio Total con Impuestos *
            </label>
            <input
              type="text"
              value={formData.total_price_with_taxes}
              onChange={(e) =>
                handleChange("total_price_with_taxes", e.target.value)
              }
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${
                  errors.total_price_with_taxes ? "var(--color-danger)" : "var(--color-border)"
                }`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="0.00"
            />
            {errors.total_price_with_taxes && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.total_price_with_taxes}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              VAT (%) *
            </label>
            <input
              type="text"
              value={formData.vat}
              onChange={(e) => handleChange("vat", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.vat ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="0.00"
            />
            {errors.vat && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.vat}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              ICO (%) *
            </label>
            <input
              type="text"
              value={formData.ico}
              onChange={(e) => handleChange("ico", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.ico ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="0.00"
            />
            {errors.ico && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.ico}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Descripción
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => handleChange("description", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${
                  errors.description ? "var(--color-danger)" : "var(--color-border)"
                }`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                minHeight: "80px",
                resize: "vertical",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa la descripción del producto (opcional)"
            />
            {errors.description && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.description}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Marca
            </label>
            <input
              type="text"
              value={formData.brand}
              onChange={(e) => handleChange("brand", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.brand ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa la marca (opcional)"
            />
            {errors.brand && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.brand}
              </p>
            )}
          </div>

          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "var(--color-text-primary)",
              }}
            >
              Modelo
            </label>
            <input
              type="text"
              value={formData.model}
              onChange={(e) => handleChange("model", e.target.value)}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${errors.model ? "var(--color-danger)" : "var(--color-border)"}`,
                borderRadius: "var(--radius-sm)",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: "var(--color-bg)",
                color: "var(--color-text-primary)",
              }}
              placeholder="Ingresa el modelo (opcional)"
            />
            {errors.model && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.model}
              </p>
            )}
          </div>

          <div
            style={{ display: "flex", gap: "1rem", justifyContent: "flex-end" }}
          >
            <button
              type="button"
              onClick={onCancel}
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-surface-hover)",
                color: "var(--color-text-primary)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-success)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              {isLoading ? "Guardando..." : product ? "Actualizar" : "Crear"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
