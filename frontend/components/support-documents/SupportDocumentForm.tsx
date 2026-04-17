"use client";

import { useState, useMemo } from "react";
import type {
  CreateSupportDocumentRequest,
  SupportDocumentPaymentCode,
  ProviderDocumentType,
} from "@/types/support-document";

interface SupportDocumentFormProps {
  onSubmit: (data: CreateSupportDocumentRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

const PAYMENT_CODES: {
  value: SupportDocumentPaymentCode;
  label: string;
}[] = [
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

const DOCUMENT_TYPES: { value: ProviderDocumentType; label: string }[] = [
  { value: "CC", label: "CC - Cédula de Ciudadanía" },
  { value: "NIT", label: "NIT" },
];

interface SimpleItem {
  quantity: number;
  description: string;
  price: number;
}

export default function SupportDocumentForm({
  onSubmit,
  onCancel,
  isLoading = false,
}: SupportDocumentFormProps) {
  const [showConfirmation, setShowConfirmation] = useState(false);
  const [pendingSubmitData, setPendingSubmitData] =
    useState<CreateSupportDocumentRequest | null>(null);
  const [formData, setFormData] = useState({
    payment_code: "cash" as SupportDocumentPaymentCode,
    provider: {
      id: "",
      document_type: "CC" as ProviderDocumentType,
      name: "",
      email: "",
    },
    items: [] as SimpleItem[],
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  const purchaseSummary = useMemo(() => {
    const totalItems = formData.items.length;
    const totalQuantity = formData.items.reduce(
      (sum, item) => sum + (item.quantity || 0),
      0
    );
    const totalAmount = formData.items.reduce(
      (sum, item) => sum + (item.quantity || 0) * (item.price || 0),
      0
    );
    return { totalItems, totalQuantity, totalAmount };
  }, [formData.items]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.provider.id.trim()) {
      newErrors["provider.id"] =
        "El número de documento del proveedor es requerido";
    }
    if (!formData.provider.name.trim()) {
      newErrors["provider.name"] = "El nombre del proveedor es requerido";
    }
    if (!formData.provider.email.trim()) {
      newErrors["provider.email"] = "El correo del proveedor es requerido";
    } else if (
      !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.provider.email)
    ) {
      newErrors["provider.email"] = "Formato de correo electrónico inválido";
    }

    if (formData.items.length === 0) {
      newErrors.items = "Se requiere al menos un artículo";
    }

    formData.items.forEach((item, index) => {
      if (!item.quantity || item.quantity <= 0) {
        newErrors[`items.${index}.quantity`] =
          "La cantidad debe ser mayor a 0";
      }
      if (!item.description.trim()) {
        newErrors[`items.${index}.description`] =
          "La descripción es requerida";
      }
      if (!item.price || item.price <= 0) {
        newErrors[`items.${index}.price`] =
          "El precio debe ser mayor a 0";
      }
    });

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const buildSubmitData = (): CreateSupportDocumentRequest => {
    return {
      payment_code: formData.payment_code,
      provider: {
        id: formData.provider.id.trim(),
        document_type: formData.provider.document_type,
        name: formData.provider.name.trim(),
        email: formData.provider.email.trim(),
      },
      items: formData.items.map((item) => ({
        quantity: item.quantity,
        description: item.description.trim(),
        price: item.price,
      })),
    };
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    const submitData = buildSubmitData();
    setPendingSubmitData(submitData);
    setShowConfirmation(true);
  };

  const handleConfirmSubmit = async () => {
    if (!pendingSubmitData) return;
    setShowConfirmation(false);
    await onSubmit(pendingSubmitData);
    setPendingSubmitData(null);
  };

  const handleCancelConfirmation = () => {
    setShowConfirmation(false);
    setPendingSubmitData(null);
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
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  const addItem = () => {
    setFormData((prev) => ({
      ...prev,
      items: [...prev.items, { quantity: 0, description: "", price: 0 }],
    }));
  };

  const removeItem = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      items: prev.items.filter((_, i) => i !== index),
    }));
  };

  const updateItem = (
    index: number,
    field: keyof SimpleItem,
    value: string | number
  ) => {
    setFormData((prev) => ({
      ...prev,
      items: prev.items.map((item, i) =>
        i === index ? { ...item, [field]: value } : item
      ),
    }));
    const errorKey = `items.${index}.${field}`;
    if (errors[errorKey]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[errorKey];
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
          Crear Documento Soporte
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Payment Code Section */}
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
              Detalles del Documento
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
                    e.target.value as SupportDocumentPaymentCode
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

          {/* Provider Section - Required */}
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
              Información del Proveedor{" "}
              <span
                style={{
                  fontSize: "0.875rem",
                  fontWeight: "normal",
                  color: "var(--color-danger)",
                }}
              >
                (Requerido)
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
                  Número de Documento *
                </label>
                <input
                  type="text"
                  value={formData.provider.id}
                  onChange={(e) => handleChange("provider.id", e.target.value)}
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["provider.id"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el NIT o CC del proveedor"
                />
                {errors["provider.id"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["provider.id"]}
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
                  Tipo de Documento *
                </label>
                <select
                  value={formData.provider.document_type}
                  onChange={(e) =>
                    handleChange(
                      "provider.document_type",
                      e.target.value as ProviderDocumentType
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
                  Nombre *
                </label>
                <input
                  type="text"
                  value={formData.provider.name}
                  onChange={(e) =>
                    handleChange("provider.name", e.target.value)
                  }
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["provider.name"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el nombre del proveedor"
                />
                {errors["provider.name"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["provider.name"]}
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
                  Correo Electrónico *
                </label>
                <input
                  type="email"
                  value={formData.provider.email}
                  onChange={(e) =>
                    handleChange("provider.email", e.target.value)
                  }
                  style={{
                    width: "100%",
                    padding: "0.75rem",
                    border: `1px solid ${
                      errors["provider.email"]
                        ? "var(--color-danger)"
                        : "var(--color-border)"
                    }`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "1rem",
                    boxSizing: "border-box",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                  placeholder="Ingresa el correo del proveedor"
                />
                {errors["provider.email"] && (
                  <p
                    style={{
                      margin: "0.25rem 0 0 0",
                      color: "var(--color-danger)",
                      fontSize: "0.875rem",
                    }}
                  >
                    {errors["provider.email"]}
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
              Artículos del Documento
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

                <div
                  style={{
                    display: "grid",
                    gridTemplateColumns: "100px 140px 1fr",
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
                      Precio *
                    </label>
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={item.price || ""}
                      onChange={(e) => {
                        const numValue =
                          e.target.value === ""
                            ? 0
                            : parseFloat(e.target.value) || 0;
                        updateItem(index, "price", numValue);
                      }}
                      style={{
                        width: "100%",
                        padding: "0.5rem",
                        border: `1px solid ${
                          errors[`items.${index}.price`]
                            ? "var(--color-danger)"
                            : "var(--color-border)"
                        }`,
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.875rem",
                        boxSizing: "border-box",
                        backgroundColor: "var(--color-surface)",
                        color: "var(--color-text-primary)",
                      }}
                      placeholder="0.00"
                    />
                    {errors[`items.${index}.price`] && (
                      <p
                        style={{
                          margin: "0.25rem 0 0 0",
                          color: "var(--color-danger)",
                          fontSize: "0.75rem",
                        }}
                      >
                        {errors[`items.${index}.price`]}
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
                      Descripción *
                    </label>
                    <input
                      type="text"
                      value={item.description}
                      onChange={(e) =>
                        updateItem(index, "description", e.target.value)
                      }
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
                        backgroundColor: "var(--color-surface)",
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

          {/* Summary */}
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
              Resumen
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
              {isLoading ? "Creando..." : "Crear Documento Soporte"}
            </button>
          </div>
        </form>

        {/* Confirmation Modal */}
        {showConfirmation && (
          <div
            style={{
              position: "fixed",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: "rgba(0, 0, 0, 0.6)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              zIndex: 1100,
            }}
            onClick={handleCancelConfirmation}
          >
            <div
              style={{
                backgroundColor: "var(--color-surface)",
                borderRadius: "var(--radius-md)",
                padding: "2rem",
                maxWidth: "420px",
                width: "90%",
                boxShadow: "var(--shadow-lg, 0 10px 25px rgba(0,0,0,0.2))",
                textAlign: "center",
              }}
              onClick={(e) => e.stopPropagation()}
            >
              <h3
                style={{
                  margin: "0 0 1rem 0",
                  fontSize: "1.25rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Confirmar Documento Soporte
              </h3>
              <p
                style={{
                  margin: "0 0 0.5rem 0",
                  fontSize: "0.95rem",
                  color: "var(--color-text-secondary)",
                }}
              >

                Estás a punto de crear un documento soporte por un valor total
                de:
              </p>
              <p
                style={{
                  margin: "0 0 0.5rem 0",
                  fontSize: "2rem",
                  fontWeight: "700",
                  color: "var(--color-primary)",
                }}
              >
                $
                {purchaseSummary.totalAmount.toLocaleString("es-CO", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                })}
              </p>
              <p
                style={{
                  margin: "0 0 1.5rem 0",
                  fontSize: "0.95rem",
                  color: "var(--color-text-secondary)",
                }}
              >
                Proveedor: <strong>{formData.provider.name}</strong> (
                {formData.provider.id})
              </p>
              <div
                style={{
                  display: "flex",
                  gap: "1rem",
                  justifyContent: "center",
                }}
              >
                <button
                  type="button"
                  onClick={handleCancelConfirmation}
                  style={{
                    padding: "0.75rem 1.5rem",
                    backgroundColor: "var(--color-surface-hover)",
                    color: "var(--color-text-primary)",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-sm)",
                    cursor: "pointer",
                    fontSize: "1rem",
                    fontWeight: "500",
                  }}
                >
                  Cancelar
                </button>
                <button
                  type="button"
                  onClick={handleConfirmSubmit}
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
                  {isLoading ? "Creando..." : "Confirmar"}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
