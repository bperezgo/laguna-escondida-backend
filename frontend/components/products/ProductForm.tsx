"use client";

import { useState, useEffect, useRef } from "react";
import type {
  Product,
  CreateProductRequest,
  UpdateProductRequest,
  ProductType,
  UnitOfMeasure,
  ProductResponsibility,
} from "@/types/product";
import { PRODUCT_TYPES, UNITS_OF_MEASURE, PREPARATION_AREAS, PRIORITY_LEVELS, requiresPricing } from "@/types/product";
import { productsApi } from "@/lib/api/products";
import { productResponsibilitiesApi } from "@/lib/api/productResponsibilities";
import { Modal, Input, Select, Textarea, Button } from "@/components/ui";

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
  const [categories, setCategories] = useState<string[]>([]);
  const [showCategorySuggestions, setShowCategorySuggestions] = useState(false);
  const categoryInputRef = useRef<HTMLInputElement>(null);
  const suggestionsRef = useRef<HTMLDivElement>(null);

  // Product Responsibility State
  const [hasResponsibility, setHasResponsibility] = useState(false);
  const [responsibilityArea, setResponsibilityArea] = useState("");
  const [responsibilityPriority, setResponsibilityPriority] = useState("0");
  const [existingResponsibilityId, setExistingResponsibilityId] = useState<string | null>(null);
  const [responsibilityLoading, setResponsibilityLoading] = useState(false);

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

  // Fetch categories on mount
  useEffect(() => {
    const fetchCategories = async () => {
      try {
        const fetchedCategories = await productsApi.getCategories();
        setCategories(fetchedCategories);
      } catch (error) {
        console.error("Error fetching categories:", error);
      }
    };
    fetchCategories();
  }, []);

  // Close suggestions when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        categoryInputRef.current &&
        !categoryInputRef.current.contains(event.target as Node) &&
        suggestionsRef.current &&
        !suggestionsRef.current.contains(event.target as Node)
      ) {
        setShowCategorySuggestions(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, []);

  // Fetch existing product responsibility when editing
  useEffect(() => {
    const fetchResponsibility = async () => {
      if (!product?.id) {
        // Reset responsibility state for new products
        setHasResponsibility(false);
        setResponsibilityArea("");
        setResponsibilityPriority("0");
        setExistingResponsibilityId(null);
        return;
      }

      try {
        setResponsibilityLoading(true);
        const responsibility = await productResponsibilitiesApi.getById(product.id);

        // If we found a responsibility, populate the form
        setHasResponsibility(true);
        setResponsibilityArea(responsibility.area);
        setResponsibilityPriority(responsibility.priority.toString());
        setExistingResponsibilityId(responsibility.id);
      } catch (error) {
        // 404 means no responsibility exists, which is fine
        // Just leave the fields empty
        setHasResponsibility(false);
        setResponsibilityArea("");
        setResponsibilityPriority("0");
        setExistingResponsibilityId(null);
      } finally {
        setResponsibilityLoading(false);
      }
    };

    fetchResponsibility();
  }, [product?.id]);

  // Filter categories based on input
  const filteredCategories = categories.filter((category) =>
    category.toLowerCase().includes(formData.category.toLowerCase())
  );

  const handleCategorySelect = (category: string) => {
    handleChange("category", category);
    setShowCategorySuggestions(false);
  };

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

    // Product Responsibility Validation
    if (hasResponsibility) {
      // Area: required, min=1, max=255
      if (!responsibilityArea.trim()) {
        newErrors.responsibility_area = "El área de preparación es requerida";
      } else if (responsibilityArea.length > 255) {
        newErrors.responsibility_area = "El área debe tener 255 caracteres o menos";
      }

      // Priority: required, must be valid level (0-3)
      if (!responsibilityPriority.trim()) {
        newErrors.responsibility_priority = "El nivel de prioridad es requerido";
      } else {
        const priority = parseInt(responsibilityPriority);
        if (isNaN(priority) || priority < 0 || priority > 3) {
          newErrors.responsibility_priority = "El nivel de prioridad debe estar entre 0 y 3";
        }
      }
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

    try {
      // First, submit the product
      await onSubmit(submitData);

      // After product is saved successfully, handle responsibility
      await handleResponsibilityOperations();
    } catch (error) {
      // Error is already handled by onSubmit
      throw error;
    }
  };

  const handleResponsibilityOperations = async () => {
    try {
      if (hasResponsibility) {
        const responsibilityData = {
          area: responsibilityArea.trim(),
          priority: parseInt(responsibilityPriority),
        };

        if (existingResponsibilityId) {
          // Update existing responsibility
          await productResponsibilitiesApi.update(
            existingResponsibilityId,
            responsibilityData
          );
        } else {
          // Create new responsibility
          await productResponsibilitiesApi.create({
            product_name: formData.name.trim(),
            ...responsibilityData,
          });
        }
      } else if (existingResponsibilityId) {
        // Delete responsibility if checkbox is unchecked
        await productResponsibilitiesApi.delete(existingResponsibilityId);
      }
    } catch (error) {
      // Log the error but don't fail the entire operation
      console.error("Error handling product responsibility:", error);
      const errorMessage =
        error instanceof Error
          ? error.message
          : "Failed to save product responsibility";

      // Show a warning to the user
      alert(
        `El producto se guardó correctamente, pero hubo un problema con la responsabilidad de preparación: ${errorMessage}`
      );
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
    <Modal
      open
      onClose={onCancel}
      title={product ? "Editar Producto" : "Crear Nuevo Producto"}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="product-form" disabled={isLoading}>
            {isLoading ? "Guardando..." : product ? "Actualizar" : "Crear"}
          </Button>
        </>
      }
    >
      <form
        id="product-form"
        onSubmit={handleSubmit}
        style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
      >
        {/* Name */}
        <Input
          label="Nombre del Producto *"
          type="text"
          value={formData.name}
          onChange={(e) => handleChange("name", e.target.value)}
          error={errors.name}
          placeholder="Ingresa el nombre del producto"
          maxLength={255}
        />

        {/* Category with Autocomplete */}
        <div style={{ position: "relative" }}>
          <Input
            ref={categoryInputRef}
            label="Categoría *"
            type="text"
            value={formData.category}
            onChange={(e) => {
              handleChange("category", e.target.value);
              setShowCategorySuggestions(true);
            }}
            onFocus={() => setShowCategorySuggestions(true)}
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                setShowCategorySuggestions(false);
              }
            }}
            error={errors.category}
            placeholder="Ingresa la categoría"
            maxLength={100}
            autoComplete="off"
          />
          {/* Category Suggestions Dropdown */}
          {showCategorySuggestions && filteredCategories.length > 0 && formData.category && (
            <div
              ref={suggestionsRef}
              style={{
                position: "absolute",
                top: "100%",
                left: 0,
                right: 0,
                backgroundColor: "var(--color-surface)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-sm)",
                boxShadow: "var(--shadow-md)",
                maxHeight: "200px",
                overflowY: "auto",
                zIndex: 10,
                marginTop: "0.25rem",
              }}
            >
              {filteredCategories.map((category, index) => (
                <div
                  key={index}
                  onClick={() => handleCategorySelect(category)}
                  style={{
                    padding: "0.75rem",
                    cursor: "pointer",
                    borderBottom:
                      index < filteredCategories.length - 1
                        ? "1px solid var(--color-border)"
                        : "none",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                    transition: "background-color var(--transition-fast)",
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = "var(--color-surface-hover)";
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = "var(--color-surface)";
                  }}
                >
                  {category}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Product Type and Unit of Measure - Side by Side */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: "1rem",
          }}
        >
          <Select
            label="Tipo de Producto *"
            value={formData.product_type}
            onChange={(e) => handleChange("product_type", e.target.value)}
            error={errors.product_type}
          >
            {PRODUCT_TYPES.map((type) => (
              <option key={type.value} value={type.value}>
                {type.label}
              </option>
            ))}
          </Select>

          <Select
            label="Unidad de Medida *"
            value={formData.unit_of_measure}
            onChange={(e) => handleChange("unit_of_measure", e.target.value)}
            error={errors.unit_of_measure}
          >
            {UNITS_OF_MEASURE.map((unit) => (
              <option key={unit.value} value={unit.value}>
                {unit.label}
              </option>
            ))}
          </Select>
        </div>

        {/* SKU */}
        <Input
          label="SKU *"
          type="text"
          value={formData.sku}
          onChange={(e) => handleChange("sku", e.target.value)}
          error={errors.sku}
          placeholder="Ingresa el SKU"
          maxLength={255}
        />

        {/* Product Responsibility Section */}
        <div
          style={{
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
            Responsabilidad de Preparación
          </h3>

          {responsibilityLoading ? (
            <p style={{ color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
              Cargando información de preparación...
            </p>
          ) : (
            <>
              {/* Checkbox */}
              <div style={{ marginBottom: "1rem" }}>
                <label
                  style={{
                    display: "flex",
                    alignItems: "center",
                    cursor: "pointer",
                    gap: "0.5rem",
                  }}
                >
                  <input
                    type="checkbox"
                    checked={hasResponsibility}
                    onChange={(e) => {
                      setHasResponsibility(e.target.checked);
                      // Clear errors when toggling
                      if (!e.target.checked) {
                        setErrors((prev) => {
                          const newErrors = { ...prev };
                          delete newErrors.responsibility_area;
                          delete newErrors.responsibility_priority;
                          return newErrors;
                        });
                      }
                    }}
                    style={{
                      width: "1.25rem",
                      height: "1.25rem",
                      cursor: "pointer",
                      accentColor: "var(--color-primary)",
                    }}
                  />
                  <span style={{ color: "var(--color-text-primary)", fontWeight: "500" }}>
                    Este producto requiere preparación
                  </span>
                </label>
              </div>

              {/* Area and Priority - Only shown when checkbox is checked */}
              {hasResponsibility && (
                <>
                  {/* Info text */}
                  <div
                    style={{
                      marginBottom: "1rem",
                      padding: "0.75rem",
                      backgroundColor: "var(--color-primary-light)",
                      borderRadius: "var(--radius-sm)",
                      border: "1px solid var(--color-primary)",
                      color: "var(--color-text-primary)",
                      fontSize: "0.875rem",
                    }}
                  >
                    Define qué área es responsable de preparar este producto y su nivel de prioridad. A mayor prioridad, menor tiempo de entrega.
                  </div>

                  <div
                    style={{
                      display: "grid",
                      gridTemplateColumns: "1fr 1fr",
                      gap: "1rem",
                    }}
                  >
                    {/* Area Dropdown */}
                    <Select
                      label="Área de Preparación *"
                      value={responsibilityArea}
                      onChange={(e) => {
                        setResponsibilityArea(e.target.value);
                        // Clear error when user changes value
                        if (errors.responsibility_area) {
                          setErrors((prev) => {
                            const newErrors = { ...prev };
                            delete newErrors.responsibility_area;
                            return newErrors;
                          });
                        }
                      }}
                      error={errors.responsibility_area}
                    >
                      <option value="">Selecciona un área</option>
                      {PREPARATION_AREAS.map((area) => (
                        <option key={area.value} value={area.value}>
                          {area.label}
                        </option>
                      ))}
                    </Select>

                    {/* Priority Dropdown */}
                    <Select
                      label="Nivel de Prioridad *"
                      value={responsibilityPriority}
                      onChange={(e) => {
                        setResponsibilityPriority(e.target.value);
                        // Clear error when user changes value
                        if (errors.responsibility_priority) {
                          setErrors((prev) => {
                            const newErrors = { ...prev };
                            delete newErrors.responsibility_priority;
                            return newErrors;
                          });
                        }
                      }}
                      error={errors.responsibility_priority}
                    >
                      {PRIORITY_LEVELS.map((level) => (
                        <option key={level.value} value={level.value.toString()}>
                          {level.label}
                        </option>
                      ))}
                    </Select>
                  </div>
                </>
              )}
            </>
          )}
        </div>

        {/* Pricing Section - Only shown for sellable products */}
        {needsPricing && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-sm)",
              border: "1px solid var(--color-border)",
              display: "flex",
              flexDirection: "column",
              gap: "1rem",
            }}
          >
            <h3
              style={{
                margin: 0,
                fontSize: "1rem",
                fontWeight: "600",
                color: "var(--color-text-secondary)",
              }}
            >
              Información de Precios
            </h3>

            {/* Price Total */}
            <Input
              label="Precio Total con Impuestos *"
              type="text"
              value={formData.total_price_with_taxes}
              onChange={(e) =>
                handleChange("total_price_with_taxes", e.target.value)
              }
              error={errors.total_price_with_taxes}
              placeholder="0.00"
            />

            {/* VAT and ICO - Side by Side */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: "1rem",
              }}
            >
              <Input
                label="IVA (%) *"
                type="text"
                value={formData.vat}
                onChange={(e) => handleChange("vat", e.target.value)}
                error={errors.vat}
                placeholder="19"
              />

              <Input
                label="ICO (%) *"
                type="text"
                value={formData.ico}
                onChange={(e) => handleChange("ico", e.target.value)}
                error={errors.ico}
                placeholder="8"
              />
            </div>
          </div>
        )}

        {/* Info message for ingredients */}
        {!needsPricing && (
          <div
            style={{
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
        <Textarea
          label="Descripción"
          value={formData.description}
          onChange={(e) => handleChange("description", e.target.value)}
          error={errors.description}
          placeholder="Ingresa la descripción del producto (opcional)"
        />
      </form>
    </Modal>
  );
}
