"use client";

import { useState, useEffect, useMemo } from "react";
import type {
  CreateElectronicInvoiceRequest,
  ElectronicInvoicePaymentCode,
  DocumentType,
  InvoiceItem,
  InvoiceAllowance,
  InvoiceTax,
} from "@/types/invoice";
import type { Product } from "@/types/product";
import { productsApi } from "@/lib/api/products";

interface InvoiceFormProps {
  onSubmit: (data: CreateElectronicInvoiceRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const PAYMENT_CODES: { value: ElectronicInvoicePaymentCode; label: string }[] =
  [
    { value: "credit_card", label: "Tarjeta de Crédito" },
    { value: "debit_card", label: "Tarjeta de Débito" },
    { value: "cash", label: "Efectivo" },
    { value: "transfer_debit_bank", label: "Transferencia Débito Bancaria" },
    { value: "transfer_credit_bank", label: "Transferencia Crédito Bancaria" },
    {
      value: "transfer_debit_interbank",
      label: "Transferencia Débito Interbancaria",
    },
  ];

const DOCUMENT_TYPES: { value: DocumentType; label: string }[] = [
  { value: "CC", label: "CC - Cédula de Ciudadanía" },
  { value: "NIT", label: "NIT" },
];

export default function InvoiceForm({
  onSubmit,
  onCancel,
  isLoading = false,
}: InvoiceFormProps) {
  const [formData, setFormData] = useState({
    payment_code: "cash" as ElectronicInvoicePaymentCode,
    customer: {
      id: "",
      document_type: "CC" as DocumentType,
      name: "",
      email: "",
    },
    items: [] as InvoiceItem[],
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [products, setProducts] = useState<Product[]>([]);
  const [productsLoading, setProductsLoading] = useState(false);
  const [itemProductSearch, setItemProductSearch] = useState<
    Record<number, string>
  >({});
  const [itemSelectedProduct, setItemSelectedProduct] = useState<
    Record<number, Product | null>
  >({});

  // Fetch products on mount
  useEffect(() => {
    const fetchProducts = async () => {
      setProductsLoading(true);
      try {
        const fetchedProducts = await productsApi.getAll();
        setProducts(fetchedProducts);
      } catch (error) {
        console.error("Error fetching products:", error);
      } finally {
        setProductsLoading(false);
      }
    };

    fetchProducts();
  }, []);

  // Create a stable dependency string for quantity and totalPriceWithTaxes
  const itemsDependency = useMemo(
    () =>
      formData.items
        .map((item) => `${item.quantity}|${item.totalPriceWithTaxes}`)
        .join("|"),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [formData.items]
  );

  // Calculate purchase summary
  const purchaseSummary = useMemo(() => {
    const totalItems = formData.items.length;
    const totalQuantity = formData.items.reduce(
      (sum, item) => sum + (item.quantity || 0),
      0
    );
    const totalAmount = formData.items.reduce(
      (sum, item) => sum + (parseFloat(item.total) || 0),
      0
    );
    return {
      totalItems,
      totalQuantity,
      totalAmount,
    };
  }, [formData.items]);

  // Auto-calculate totals when quantity or totalPriceWithTaxes changes
  useEffect(() => {
    formData.items.forEach((item, index) => {
      const quantity = item.quantity || 0;
      const totalPriceWithTaxes = parseFloat(item.totalPriceWithTaxes) || 0;
      const calculatedTotal = (quantity * totalPriceWithTaxes).toFixed(2);

      // Only update if the total has changed to avoid infinite loops
      if (item.total !== calculatedTotal) {
        setFormData((prev) => ({
          ...prev,
          items: prev.items.map((it, i) =>
            i === index ? { ...it, total: calculatedTotal } : it
          ),
        }));
      }
    });
  }, [itemsDependency, formData.items.length]);

  // Filter products based on search query for a specific item
  const getFilteredProducts = (itemIndex: number): Product[] => {
    const searchQuery = itemProductSearch[itemIndex] || "";
    if (!searchQuery.trim()) {
      return products;
    }
    const query = searchQuery.toLowerCase();
    return products.filter(
      (product) =>
        product.name.toLowerCase().includes(query) ||
        product.sku.toLowerCase().includes(query) ||
        (product.description &&
          product.description.toLowerCase().includes(query)) ||
        (product.brand && product.brand.toLowerCase().includes(query)) ||
        (product.model && product.model.toLowerCase().includes(query))
    );
  };

  // Handle product selection for an item
  const handleProductSelect = (itemIndex: number, product: Product) => {
    setItemSelectedProduct((prev) => ({
      ...prev,
      [itemIndex]: product,
    }));
    setItemProductSearch((prev) => ({
      ...prev,
      [itemIndex]: product.name,
    }));

    // Auto-populate item fields from product
    updateItem(itemIndex, "product_id", product.id);
    updateItem(itemIndex, "description", product.description || product.name);
    updateItem(
      itemIndex,
      "totalPriceWithTaxes",
      product.total_price_with_taxes.toString()
    );
    updateItem(itemIndex, "brand", product.brand || "");
    updateItem(itemIndex, "model", product.model || "");
    updateItem(itemIndex, "code", product.sku);

    // Note: Total will be automatically recalculated by useEffect when totalPriceWithTaxes is set
  };

  // Handle clearing product selection for an item
  const handleClearProduct = (itemIndex: number) => {
    setItemSelectedProduct((prev) => {
      const newState = { ...prev };
      delete newState[itemIndex];
      return newState;
    });
    setItemProductSearch((prev) => ({
      ...prev,
      [itemIndex]: "",
    }));
    // Clear auto-populated fields
    updateItem(itemIndex, "description", "");
    updateItem(itemIndex, "totalPriceWithTaxes", "");
    updateItem(itemIndex, "brand", "");
    updateItem(itemIndex, "model", "");
    updateItem(itemIndex, "code", "");
    updateItem(itemIndex, "total", "");
  };

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Validate customer (optional - if any field is filled, all are required)
    const hasCustomerData =
      formData.customer.id.trim() ||
      formData.customer.name.trim() ||
      formData.customer.email.trim();

    if (hasCustomerData) {
      if (!formData.customer.id.trim()) {
        newErrors["customer.id"] =
          "El número de documento es requerido cuando se proporciona información del cliente";
      }
      if (!formData.customer.name.trim()) {
        newErrors["customer.name"] =
          "El nombre del cliente es requerido cuando se proporciona información del cliente";
      }
      if (!formData.customer.email.trim()) {
        newErrors["customer.email"] =
          "El correo del cliente es requerido cuando se proporciona información del cliente";
      } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.customer.email)) {
        newErrors["customer.email"] = "Formato de correo electrónico inválido";
      }
    }

    // Validate items
    if (formData.items.length === 0) {
      newErrors.items = "Se requiere al menos un artículo";
    }

    formData.items.forEach((item, index) => {
      if (!item.quantity || item.quantity <= 0) {
        newErrors[`items.${index}.quantity`] = "La cantidad debe ser mayor a 0";
      }
      if (!item.totalPriceWithTaxes.trim()) {
        newErrors[`items.${index}.totalPriceWithTaxes`] =
          "El precio total con impuestos es requerido";
      }
      if (!item.description.trim()) {
        newErrors[`items.${index}.description`] = "La descripción es requerida";
      }
    });

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    // Only include customer if at least one field is filled
    const hasCustomerData =
      formData.customer.id.trim() ||
      formData.customer.name.trim() ||
      formData.customer.email.trim();

    const submitData: CreateElectronicInvoiceRequest = {
      payment_code: formData.payment_code,
      ...(hasCustomerData && {
        customer: {
          id: formData.customer.id.trim(),
          document_type: formData.customer.document_type,
          name: formData.customer.name.trim(),
          email: formData.customer.email.trim(),
        },
      }),
      items: formData.items.map((item) => ({
        product_id: item.product_id.trim(),
        quantity: item.quantity,
        totalPriceWithTaxes: item.totalPriceWithTaxes.trim(),
        total: item.total.trim(),
        description: item.description.trim(),
        brand: item.brand.trim(),
        model: item.model.trim(),
        code: item.code.trim(),
        allowance: item.allowance || [],
        taxes: item.taxes || [],
      })),
    };

    await onSubmit(submitData);
  };

  const handleChange = (field: string, value: string) => {
    const keys = field.split(".");
    if (keys.length === 1) {
      setFormData((prev) => ({ ...prev, [field]: value }));
    } else if (keys.length === 2) {
      setFormData((prev) => ({
        ...prev,
        [keys[0]]: {
          ...(prev[keys[0] as keyof typeof prev] as any),
          [keys[1]]: value,
        },
      }));
    }
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const addItem = () => {
    const newIndex = formData.items.length;
    setFormData((prev) => ({
      ...prev,
      items: [
        ...prev.items,
        {
          product_id: "",
          quantity: 0,
          totalPriceWithTaxes: "",
          total: "",
          description: "",
          brand: "",
          model: "",
          code: "",
        },
      ],
    }));
    // Initialize product search state for new item
    setItemProductSearch((prev) => ({
      ...prev,
      [newIndex]: "",
    }));
    setItemSelectedProduct((prev) => ({
      ...prev,
      [newIndex]: null,
    }));
  };

  const removeItem = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      items: prev.items.filter((_, i) => i !== index),
    }));
    // Clean up product search state
    setItemProductSearch((prev) => {
      const newState = { ...prev };
      delete newState[index];
      // Reindex remaining items
      const reindexed: Record<number, string> = {};
      Object.keys(newState).forEach((key) => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = newState[oldIndex];
        } else if (oldIndex < index) {
          reindexed[oldIndex] = newState[oldIndex];
        }
      });
      return reindexed;
    });
    setItemSelectedProduct((prev) => {
      const newState = { ...prev };
      delete newState[index];
      // Reindex remaining items
      const reindexed: Record<number, Product | null> = {};
      Object.keys(newState).forEach((key) => {
        const oldIndex = parseInt(key);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = newState[oldIndex];
        } else if (oldIndex < index) {
          reindexed[oldIndex] = newState[oldIndex];
        }
      });
      return reindexed;
    });
  };

  const updateItem = (
    index: number,
    field: keyof InvoiceItem,
    value: string | number
  ) => {
    setFormData((prev) => ({
      ...prev,
      items: prev.items.map((item, i) =>
        i === index ? { ...item, [field]: value } : item
      ),
    }));
    // Clear error
    const errorKey = `items.${index}.${field}`;
    if (errors[errorKey]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[errorKey];
        return newErrors;
      });
    }
  };

  const calculateItemTotal = (index: number) => {
    const item = formData.items[index];
    const quantity = item.quantity || 0;
    const totalPriceWithTaxes = parseFloat(item.totalPriceWithTaxes) || 0;
    const total = (quantity * totalPriceWithTaxes).toFixed(2);
    updateItem(index, "total", total);
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
        overflowY: "auto",
      }}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          padding: "2rem",
          maxWidth: "900px",
          width: "100%",
          maxHeight: "95vh",
          overflowY: "auto",
          boxShadow: "var(--shadow-xl)",
          margin: "auto",
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
          Crear Factura Electrónica
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Invoice Details Section */}
          <div
            style={{
              marginBottom: "2rem",
              paddingBottom: "1.5rem",
              borderBottom: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                marginTop: 0,
                marginBottom: "1rem",
                fontSize: "1.2rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Detalles de Factura
            </h3>

            <div>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontWeight: "500",
                  color: "var(--color-text-primary)",
                }}
              >
                Código de Pago *
              </label>
              <select
                value={formData.payment_code}
                onChange={(e) =>
                  handleChange(
                    "payment_code",
                    e.target.value as ElectronicInvoicePaymentCode
                  )
                }
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
                {PAYMENT_CODES.map((code) => (
                  <option key={code.value} value={code.value}>
                    {code.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Customer Section */}
          <div
            style={{
              marginBottom: "2rem",
              paddingBottom: "1.5rem",
              borderBottom: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                marginTop: 0,
                marginBottom: "1rem",
                fontSize: "1.2rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Información del Cliente{" "}
              <span
                style={{
                  fontSize: "0.875rem",
                  fontWeight: "normal",
                  color: "var(--color-text-muted)",
                }}
              >
                (Opcional)
              </span>
            </h3>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: "1rem",
                marginBottom: "1rem",
              }}
            >
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontWeight: "500",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Número de Documento
                </label>
                <input
                  type="text"
                  value={formData.customer.id}
                  onChange={(e) => handleChange("customer.id", e.target.value)}
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["customer.id"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el número de documento"
                />
                {errors["customer.id"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["customer.id"]}
                  </p>
                )}
              </div>

              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontWeight: "500",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Tipo de Documento
                </label>
                <select
                  value={formData.customer.document_type}
                  onChange={(e) =>
                    handleChange(
                      "customer.document_type",
                      e.target.value as DocumentType
                    )
                  }
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
                  {DOCUMENT_TYPES.map((type) => (
                    <option key={type.value} value={type.value}>
                      {type.label}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: "1rem",
              }}
            >
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontWeight: "500",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Nombre
                </label>
                <input
                  type="text"
                  value={formData.customer.name}
                  onChange={(e) =>
                    handleChange("customer.name", e.target.value)
                  }
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["customer.name"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el nombre del cliente"
                />
                {errors["customer.name"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["customer.name"]}
                  </p>
                )}
              </div>

              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontWeight: "500",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Correo Electrónico
                </label>
                <input
                  type="email"
                  value={formData.customer.email}
                  onChange={(e) =>
                    handleChange("customer.email", e.target.value)
                  }
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["customer.email"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el correo del cliente"
                />
                {errors["customer.email"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["customer.email"]}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Items Section */}
          <div
            style={{
              marginBottom: "2rem",
              paddingBottom: "1.5rem",
              borderBottom: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                margin: 0,
                marginBottom: "1rem",
                fontSize: "1.2rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Artículos de la Factura
            </h3>
            {errors.items && (
              <p
                style={{
                  margin: "0 0 1rem 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.items}
              </p>
            )}
            {formData.items.map((item, index) => (
              <div
                key={index}
                style={{
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  padding: "1rem",
                  marginBottom: "1rem",
                  backgroundColor: "var(--color-bg)",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    marginBottom: "1rem",
                  }}
                >
                  <h4
                    style={{
                      margin: 0,
                      fontSize: "1rem",
                      fontWeight: "600",
                      color: "var(--color-text-primary)",
                    }}
                  >
                    Artículo {index + 1}
                  </h4>
                  <button
                    type="button"
                    onClick={() => removeItem(index)}
                    style={{
                      padding: "0.25rem 0.75rem",
                      backgroundColor: "var(--color-danger)",
                      color: "white",
                      border: "none",
                      borderRadius: "var(--radius-sm)",
                      cursor: "pointer",
                      fontSize: "0.875rem",
                    }}
                  >
                    Eliminar
                  </button>
                </div>

                {/* Product Selector */}
                <div style={{ marginBottom: "1rem", position: "relative" }}>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.5rem",
                      fontWeight: "500",
                      color: "var(--color-text-primary)",
                      fontSize: "0.875rem",
                    }}
                  >
                    Seleccionar Producto
                  </label>
                  <div style={{ position: "relative" }}>
                    <input
                      type="text"
                      value={itemProductSearch[index] || ""}
                      onChange={(e) => {
                        setItemProductSearch((prev) => ({
                          ...prev,
                          [index]: e.target.value,
                        }));
                      }}
                      onFocus={() => {
                        // Ensure search is visible when focused
                        if (!itemProductSearch[index]) {
                          setItemProductSearch((prev) => ({
                            ...prev,
                            [index]: "",
                          }));
                        }
                      }}
                      placeholder={
                        productsLoading
                          ? "Cargando productos..."
                          : "Buscar y seleccionar un producto"
                      }
                      disabled={productsLoading}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: "var(--color-surface)",
                        color: "var(--color-text-primary)",
                      }}
                    />
                    {!itemSelectedProduct[index] &&
                      itemProductSearch[index] !== undefined &&
                      itemProductSearch[index] !== "" &&
                      getFilteredProducts(index).length > 0 && (
                        <div
                          style={{
                            position: "absolute",
                            top: "100%",
                            left: 0,
                            right: 0,
                            backgroundColor: "var(--color-surface)",
                            border: "1px solid var(--color-border)",
                            borderRadius: "var(--radius-sm)",
                            maxHeight: "200px",
                            overflowY: "auto",
                            zIndex: 1000,
                            marginTop: "0.25rem",
                            boxShadow: "var(--shadow-lg)",
                          }}
                        >
                          {getFilteredProducts(index).map((product) => (
                            <div
                              key={product.id}
                              onClick={() =>
                                handleProductSelect(index, product)
                              }
                              style={{
                                padding: "0.75rem",
                                cursor: "pointer",
                                borderBottom: "1px solid var(--color-border)",
                                transition:
                                  "background-color var(--transition-fast)",
                              }}
                              onMouseEnter={(e) => {
                                e.currentTarget.style.backgroundColor =
                                  "var(--color-surface-hover)";
                              }}
                              onMouseLeave={(e) => {
                                e.currentTarget.style.backgroundColor =
                                  "var(--color-surface)";
                              }}
                            >
                              <div
                                style={{
                                  fontWeight: "500",
                                  fontSize: "0.875rem",
                                  marginBottom: "0.25rem",
                                  color: "var(--color-text-primary)",
                                }}
                              >
                                {product.name}
                              </div>
                              <div
                                style={{
                                  fontSize: "0.75rem",
                                  color: "var(--color-text-muted)",
                                }}
                              >
                                SKU: {product.sku} | Price: $
                                {product.unit_price}
                                {product.description &&
                                  ` | ${product.description.substring(0, 50)}${
                                    product.description.length > 50 ? "..." : ""
                                  }`}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                  </div>

                  {/* Display selected product information */}
                  {itemSelectedProduct[index] && (
                    <div
                      style={{
                        marginTop: "0.75rem",
                        padding: "0.75rem",
                        backgroundColor: "var(--color-primary-light)",
                        borderRadius: "var(--radius-sm)",
                        border: "1px solid var(--color-primary)",
                      }}
                    >
                      <div
                        style={{
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center",
                          marginBottom: "0.5rem",
                        }}
                      >
                        <div
                          style={{
                            fontSize: "0.875rem",
                            fontWeight: "500",
                            color: "var(--color-primary)",
                          }}
                        >
                          Producto Seleccionado:
                        </div>
                        <button
                          type="button"
                          onClick={() => handleClearProduct(index)}
                          style={{
                            padding: "0.25rem 0.5rem",
                            backgroundColor: "var(--color-danger)",
                            color: "white",
                            border: "none",
                            borderRadius: "var(--radius-sm)",
                            cursor: "pointer",
                            fontSize: "0.75rem",
                          }}
                          title="Limpiar selección de producto"
                        >
                          Limpiar
                        </button>
                      </div>
                      <div
                        style={{
                          fontSize: "0.875rem",
                          lineHeight: "1.6",
                          color: "var(--color-text-primary)",
                        }}
                      >
                        <div>
                          <strong>Name:</strong>{" "}
                          {itemSelectedProduct[index]!.name}
                        </div>
                        <div>
                          <strong>SKU:</strong>{" "}
                          {itemSelectedProduct[index]!.sku}
                        </div>
                        <div>
                          <strong>Unit Price with Taxes:</strong> $
                          {itemSelectedProduct[index]!.total_price_with_taxes}
                        </div>
                        {itemSelectedProduct[index]!.description && (
                          <div>
                            <strong>Description:</strong>{" "}
                            {itemSelectedProduct[index]!.description}
                          </div>
                        )}
                        {itemSelectedProduct[index]!.brand && (
                          <div>
                            <strong>Brand:</strong>{" "}
                            {itemSelectedProduct[index]!.brand}
                          </div>
                        )}
                        {itemSelectedProduct[index]!.model && (
                          <div>
                            <strong>Model:</strong>{" "}
                            {itemSelectedProduct[index]!.model}
                          </div>
                        )}
                        <div>
                          <strong>Category:</strong>{" "}
                          {itemSelectedProduct[index]!.category}
                        </div>
                        <div>
                          <strong>IVA:</strong>{" "}
                          {itemSelectedProduct[index]!.vat}%
                        </div>
                        <div>
                          <strong>ICO:</strong>{" "}
                          {itemSelectedProduct[index]!.ico}%
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr 1fr",
                    gap: "1rem",
                    marginBottom: "1rem",
                  }}
                >
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Cantidad *
                    </label>
                    <input
                      type="number"
                      min="0"
                      step="1"
                      value={item.quantity || ""}
                      onChange={(e) => {
                        const numValue =
                          e.target.value === ""
                            ? 0
                            : parseInt(e.target.value, 10) || 0;
                        updateItem(index, "quantity", numValue);
                        calculateItemTotal(index);
                      }}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: `1px solid ${
                          errors[`items.${index}.quantity`]
                            ? "var(--color-danger)"
                            : "var(--color-border)"
                        }`,
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: "var(--color-surface)",
                        color: "var(--color-text-primary)",
                      }}
                      placeholder="0"
                    />
                    {errors[`items.${index}.quantity`] && (
                      <p
                        style={{
                          margin: "0.25rem 0 0 0",
                          color: "var(--color-danger)",
                          fontSize: "0.75rem",
                        }}
                      >
                        {errors[`items.${index}.quantity`]}
                      </p>
                    )}
                  </div>
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Precio Total con Impuestos *{" "}
                      {itemSelectedProduct[index] && (
                        <span
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--color-text-muted)",
                            fontWeight: "normal",
                          }}
                        >
                          (del producto)
                        </span>
                      )}
                    </label>
                    <input
                      type="text"
                      value={item.totalPriceWithTaxes}
                      onChange={
                        itemSelectedProduct[index]
                          ? undefined
                          : (e) => {
                              updateItem(
                                index,
                                "totalPriceWithTaxes",
                                e.target.value
                              );
                              calculateItemTotal(index);
                            }
                      }
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: `1px solid ${
                          errors[`items.${index}.totalPriceWithTaxes`]
                            ? "var(--color-danger)"
                            : "var(--color-border)"
                        }`,
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: itemSelectedProduct[index]
                          ? "var(--color-surface-hover)"
                          : "var(--color-surface)",
                        color: "var(--color-text-primary)",
                        cursor: itemSelectedProduct[index]
                          ? "not-allowed"
                          : "text",
                      }}
                      placeholder="0.00"
                    />
                    {errors[`items.${index}.totalPriceWithTaxes`] && (
                      <p
                        style={{
                          margin: "0.25rem 0 0 0",
                          color: "var(--color-danger)",
                          fontSize: "0.75rem",
                        }}
                      >
                        {errors[`items.${index}.totalPriceWithTaxes`]}
                      </p>
                    )}
                  </div>
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Total
                    </label>
                    <input
                      type="text"
                      value={item.total}
                      readOnly
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: "var(--color-surface-hover)",
                        color: "var(--color-text-primary)",
                      }}
                    />
                  </div>
                </div>
                <div style={{ marginBottom: "1rem" }}>
                  <label
                    style={{
                      display: "block",
                      marginBottom: "0.5rem",
                      fontWeight: "500",
                      color: "var(--color-text-primary)",
                      fontSize: "0.875rem",
                    }}
                  >
                    Descripción *{" "}
                    {itemSelectedProduct[index] && (
                      <span
                        style={{
                          fontSize: "0.75rem",
                          color: "var(--color-text-muted)",
                          fontWeight: "normal",
                        }}
                      >
                        (del producto)
                      </span>
                    )}
                  </label>
                  <input
                    type="text"
                    value={item.description}
                    onChange={(e) =>
                      updateItem(index, "description", e.target.value)
                    }
                    readOnly={!!itemSelectedProduct[index]}
                    style={{
                      width: "100%",
                      padding: "0.5rem",
                      border: `1px solid ${
                        errors[`items.${index}.description`]
                          ? "var(--color-danger)"
                          : "var(--color-border)"
                      }`,
                      borderRadius: "var(--radius-sm)",
                      fontSize: "0.875rem",
                      boxSizing: "border-box",
                      backgroundColor: itemSelectedProduct[index]
                        ? "var(--color-surface-hover)"
                        : "var(--color-surface)",
                      color: "var(--color-text-primary)",
                    }}
                    placeholder="Ingresa la descripción del artículo"
                  />
                  {errors[`items.${index}.description`] && (
                    <p
                      style={{
                        margin: "0.25rem 0 0 0",
                        color: "var(--color-danger)",
                        fontSize: "0.75rem",
                      }}
                    >
                      {errors[`items.${index}.description`]}
                    </p>
                  )}
                </div>
                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr 1fr",
                    gap: "1rem",
                  }}
                >
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Marca{" "}
                      {itemSelectedProduct[index] && (
                        <span
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--color-text-muted)",
                            fontWeight: "normal",
                          }}
                        >
                          (del producto)
                        </span>
                      )}
                    </label>
                    <input
                      type="text"
                      value={item.brand}
                      onChange={
                        itemSelectedProduct[index]
                          ? undefined
                          : (e) => updateItem(index, "brand", e.target.value)
                      }
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: itemSelectedProduct[index]
                          ? "var(--color-surface-hover)"
                          : "var(--color-surface)",
                        color: "var(--color-text-primary)",
                        cursor: itemSelectedProduct[index]
                          ? "not-allowed"
                          : "text",
                      }}
                      placeholder="Ingresa la marca"
                    />
                  </div>
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Modelo{" "}
                      {itemSelectedProduct[index] && (
                        <span
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--color-text-muted)",
                            fontWeight: "normal",
                          }}
                        >
                          (del producto)
                        </span>
                      )}
                    </label>
                    <input
                      type="text"
                      value={item.model}
                      onChange={
                        itemSelectedProduct[index]
                          ? undefined
                          : (e) => updateItem(index, "model", e.target.value)
                      }
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: itemSelectedProduct[index]
                          ? "var(--color-surface-hover)"
                          : "var(--color-surface)",
                        color: "var(--color-text-primary)",
                        cursor: itemSelectedProduct[index]
                          ? "not-allowed"
                          : "text",
                      }}
                      placeholder="Ingresa el modelo"
                    />
                  </div>
                  <div>
                    <label
                      style={{
                        display: "block",
                        marginBottom: "0.5rem",
                        fontWeight: "500",
                        color: "var(--color-text-primary)",
                        fontSize: "0.875rem",
                      }}
                    >
                      Código{" "}
                      {itemSelectedProduct[index] && (
                        <span
                          style={{
                            fontSize: "0.75rem",
                            color: "var(--color-text-muted)",
                            fontWeight: "normal",
                          }}
                        >
                          (del producto)
                        </span>
                      )}
                    </label>
                    <input
                      type="text"
                      value={item.code}
                      onChange={
                        itemSelectedProduct[index]
                          ? undefined
                          : (e) => updateItem(index, "code", e.target.value)
                      }
                      readOnly={!!itemSelectedProduct[index]}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: "1px solid var(--color-border)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: itemSelectedProduct[index]
                          ? "var(--color-surface-hover)"
                          : "var(--color-surface)",
                        color: "var(--color-text-primary)",
                        cursor: itemSelectedProduct[index]
                          ? "not-allowed"
                          : "text",
                      }}
                      placeholder="Ingresa el código"
                    />
                  </div>
                </div>
              </div>
            ))}
            <button
              type="button"
              onClick={addItem}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-primary)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                width: "100%",
                marginTop: "0.5rem",
              }}
            >
              + Agregar Artículo
            </button>
          </div>

          {/* Purchase Summary */}
          <div
            style={{
              marginBottom: "2rem",
              padding: "1.5rem",
              backgroundColor: "var(--color-surface-hover)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                margin: 0,
                marginBottom: "1rem",
                fontSize: "1.2rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Resumen de Compra
            </h3>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(3, 1fr)",
                gap: "1rem",
              }}
            >
              <div
                style={{
                  textAlign: "center",
                  padding: "1rem",
                  backgroundColor: "var(--color-surface)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <div
                  style={{
                    fontSize: "0.875rem",
                    color: "var(--color-text-muted)",
                    marginBottom: "0.5rem",
                  }}
                >
                  Artículos
                </div>
                <div
                  style={{
                    fontSize: "1.5rem",
                    fontWeight: "700",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {purchaseSummary.totalItems}
                </div>
              </div>
              <div
                style={{
                  textAlign: "center",
                  padding: "1rem",
                  backgroundColor: "var(--color-surface)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <div
                  style={{
                    fontSize: "0.875rem",
                    color: "var(--color-text-muted)",
                    marginBottom: "0.5rem",
                  }}
                >
                  Cantidad Total
                </div>
                <div
                  style={{
                    fontSize: "1.5rem",
                    fontWeight: "700",
                    color: "var(--color-text-primary)",
                  }}
                >
                  {purchaseSummary.totalQuantity}
                </div>
              </div>
              <div
                style={{
                  textAlign: "center",
                  padding: "1rem",
                  backgroundColor: "var(--color-primary-light)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-primary)",
                }}
              >
                <div
                  style={{
                    fontSize: "0.875rem",
                    color: "var(--color-primary)",
                    marginBottom: "0.5rem",
                    fontWeight: "500",
                  }}
                >
                  Total a Pagar
                </div>
                <div
                  style={{
                    fontSize: "1.5rem",
                    fontWeight: "700",
                    color: "var(--color-primary)",
                  }}
                >
                  $
                  {purchaseSummary.totalAmount.toLocaleString("es-CO", {
                    minimumFractionDigits: 2,
                    maximumFractionDigits: 2,
                  })}
                </div>
              </div>
            </div>
          </div>

          {/* Form Actions */}
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
              {isLoading ? "Creando..." : "Crear Factura"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
