"use client";

import { useState, useEffect } from "react";
import type {
  Product,
  CreateProductRequest,
  UpdateProductRequest,
  ProductType,
  UnitOfMeasure,
} from "@/types/product";
import { PRODUCT_TYPES, UNITS_OF_MEASURE, requiresPricing } from "@/types/product";

interface ProductFormProps {
  product?: Product | null;
  onSubmit: (
    data: CreateProductRequest | UpdateProductRequest
  ) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const fractionToPercentage = (value: string | undefined): string => {
  if (!value) return "";
  const num = parseFloat(value);
  if (isNaN(num)) return "";
  return (num * 100).toString();
};

export default function ProductForm({
  product,
  onSubmit,
  onCancel,
  isLoading = false,
}: ProductFormProps) {
  const [formData, setFormData] = useState({
    name: product?.name || "",
    category: product?.category || "",
    product_type: (product?.product_type || "SELLABLE") as ProductType,
    unit_of_measure: (product?.unit_of_measure || "unit") as UnitOfMeasure,
    vat: product ? fractionToPercentage(product.vat) : "",
    ico: product ? fractionToPercentage(product.ico) : "",
    description: product?.description || "",
    sku: product?.sku || "",
    total_price_with_taxes: product?.total_price_with_taxes?.toString() || "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const needsPricing = requiresPricing(formData.product_type);

  useEffect(() => {
    if (product) {
      setFormData({
        name: product.name || "",
        category: product.category || "",
        product_type: product.product_type || "SELLABLE",
        unit_of_measure: product.unit_of_measure || "unit",
        vat: fractionToPercentage(product.vat),
        ico: fractionToPercentage(product.ico),
        description: product.description || "",
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

    // Product Type: required
    if (!formData.product_type) {
      newErrors.product_type = "El tipo de producto es requerido";
    }

    // Unit of Measure: required
    if (!formData.unit_of_measure) {
      newErrors.unit_of_measure = "La unidad de medida es requerida";
    }

    // SKU: required, min=1, max=255
    if (!formData.sku.trim()) {
      newErrors.sku = "El SKU es requerido";
    } else if (formData.sku.length > 255) {
      newErrors.sku = "El SKU debe tener 255 caracteres o menos";
    }

    // Price fields only required if product type requires pricing
    if (needsPricing) {
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
    const submitData: CreateProductRequest | UpdateProductRequest = {
      name: formData.name.trim(),
      category: formData.category.trim(),
      product_type: formData.product_type,
      unit_of_measure: formData.unit_of_measure,
      sku: formData.sku.trim(),
    };

    // Add optional description if provided
    if (formData.description.trim()) {
      submitData.description = formData.description.trim();
    }

    // Add price fields only if product type requires pricing
    if (needsPricing) {
      submitData.vat = formData.vat.trim();
      submitData.ico = formData.ico.trim();
      submitData.taxes_format = "percentage";
      submitData.total_price_with_taxes = formData.total_price_with_taxes.trim();
    }

    await onSubmit(submitData);
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

  const inputStyle = (hasError: boolean) => ({
    width: "100%",
    padding: "0.75rem",
    border: `1px solid ${hasError ? "var(--color-danger)" : "var(--color-border)"}`,
    borderRadius: "var(--radius-sm)",
    fontSize: "1rem",
    boxSizing: "border-box" as const,
    backgroundColor: "var(--color-bg)",
    color: "var(--color-text-primary)",
  });

  const labelStyle = {
    display: "block",
    marginBottom: "0.5rem",
    fontWeight: "500",
    color: "var(--color-text-primary)",
  };

  const errorStyle = {
    margin: "0.25rem 0 0 0",
    color: "var(--color-danger)",
    fontSize: "0.875rem",
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
          maxWidth: "600px",
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
          {/* Name */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Nombre del Producto *</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => handleChange("name", e.target.value)}
              style={inputStyle(!!errors.name)}
              placeholder="Ingresa el nombre del producto"
              maxLength={255}
            />
            {errors.name && <p style={errorStyle}>{errors.name}</p>}
          </div>

          {/* Category */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Categoría *</label>
            <input
              type="text"
              value={formData.category}
              onChange={(e) => handleChange("category", e.target.value)}
              style={inputStyle(!!errors.category)}
              placeholder="Ingresa la categoría"
              maxLength={100}
            />
            {errors.category && <p style={errorStyle}>{errors.category}</p>}
          </div>

          {/* Product Type and Unit of Measure - Side by Side */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "1rem",
              marginBottom: "1.5rem",
            }}
          >
            <div>
              <label style={labelStyle}>Tipo de Producto *</label>
              <select
                value={formData.product_type}
                onChange={(e) => handleChange("product_type", e.target.value)}
                style={inputStyle(!!errors.product_type)}
              >
                {PRODUCT_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </select>
              {errors.product_type && (
                <p style={errorStyle}>{errors.product_type}</p>
              )}
            </div>

            <div>
              <label style={labelStyle}>Unidad de Medida *</label>
              <select
                value={formData.unit_of_measure}
                onChange={(e) => handleChange("unit_of_measure", e.target.value)}
                style={inputStyle(!!errors.unit_of_measure)}
              >
                {UNITS_OF_MEASURE.map((unit) => (
                  <option key={unit.value} value={unit.value}>
                    {unit.label}
                  </option>
                ))}
              </select>
              {errors.unit_of_measure && (
                <p style={errorStyle}>{errors.unit_of_measure}</p>
              )}
            </div>
          </div>

          {/* SKU */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>SKU *</label>
            <input
              type="text"
              value={formData.sku}
              onChange={(e) => handleChange("sku", e.target.value)}
              style={inputStyle(!!errors.sku)}
              placeholder="Ingresa el SKU"
              maxLength={255}
            />
            {errors.sku && <p style={errorStyle}>{errors.sku}</p>}
          </div>

          {/* Pricing Section - Only shown for sellable products */}
          {needsPricing && (
            <>
              <div
                style={{
                  marginBottom: "1.5rem",
                  padding: "1rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <h3
                  style={{
                    margin: "0 0 1rem 0",
                    fontSize: "1rem",
                    fontWeight: "600",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  Información de Precios
                </h3>

                {/* Price Total */}
                <div style={{ marginBottom: "1rem" }}>
                  <label style={labelStyle}>Precio Total con Impuestos *</label>
                  <input
                    type="text"
                    value={formData.total_price_with_taxes}
                    onChange={(e) =>
                      handleChange("total_price_with_taxes", e.target.value)
                    }
                    style={inputStyle(!!errors.total_price_with_taxes)}
                    placeholder="0.00"
                  />
                  {errors.total_price_with_taxes && (
                    <p style={errorStyle}>{errors.total_price_with_taxes}</p>
                  )}
                </div>

                {/* VAT and ICO - Side by Side */}
                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr",
                    gap: "1rem",
                  }}
                >
                  <div>
                    <label style={labelStyle}>IVA (%) *</label>
                    <input
                      type="text"
                      value={formData.vat}
                      onChange={(e) => handleChange("vat", e.target.value)}
                      style={inputStyle(!!errors.vat)}
                      placeholder="19"
                    />
                    {errors.vat && <p style={errorStyle}>{errors.vat}</p>}
                  </div>

                  <div>
                    <label style={labelStyle}>ICO (%) *</label>
                    <input
                      type="text"
                      value={formData.ico}
                      onChange={(e) => handleChange("ico", e.target.value)}
                      style={inputStyle(!!errors.ico)}
                      placeholder="8"
                    />
                    {errors.ico && <p style={errorStyle}>{errors.ico}</p>}
                  </div>
                </div>
              </div>
            </>
          )}

          {/* Info message for ingredients */}
          {!needsPricing && (
            <div
              style={{
                marginBottom: "1.5rem",
                padding: "1rem",
                backgroundColor: "var(--color-primary-light)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid var(--color-primary)",
                color: "var(--color-text-primary)",
                fontSize: "0.875rem",
              }}
            >
              Los ingredientes no requieren información de precios ya que no se venden directamente.
            </div>
          )}

          {/* Description */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Descripción</label>
            <textarea
              value={formData.description}
              onChange={(e) => handleChange("description", e.target.value)}
              style={{
                ...inputStyle(!!errors.description),
                minHeight: "80px",
                resize: "vertical",
              }}
              placeholder="Ingresa la descripción del producto (opcional)"
            />
            {errors.description && (
              <p style={errorStyle}>{errors.description}</p>
            )}
          </div>

          {/* Buttons */}
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
