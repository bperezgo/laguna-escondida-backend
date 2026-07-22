"use client";

import { useState, useMemo } from "react";
import type {
  CreateSupportDocumentRequest,
  SupportDocumentPaymentCode,
  ProviderDocumentType,
} from "@/types/support-document";
import { Modal, Input, Select, Button, Table } from "@/components/ui";

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
    // No default: the user must consciously pick a payment code before creating
    // the support document.
    payment_code: "" as SupportDocumentPaymentCode | "",
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

    if (!formData.payment_code) {
      newErrors.payment_code = "Selecciona un código de pago";
    }

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
      } else if (item.description.trim().length < 5) {
        newErrors[`items.${index}.description`] =
          "La descripción debe tener al menos 5 caracteres";
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
      // validate() guarantees a non-empty selection before we reach here.
      payment_code: formData.payment_code as SupportDocumentPaymentCode,
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

  const sectionHeadingStyle = {
    marginTop: 0,
    marginBottom: "1rem",
    fontSize: "1.2rem",
    fontWeight: 600,
    color: "var(--color-text-primary)",
  } as const;

  return (
    <>
      <Modal
        open
        onClose={onCancel}
        title="Crear Documento Soporte"
        size="lg"
        footer={
          <>
            <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
              Cancelar
            </Button>
            <Button
              type="submit"
              form="support-document-form"
              disabled={isLoading}
            >
              {isLoading ? "Creando..." : "Crear Documento Soporte"}
            </Button>
          </>
        }
      >
        <form
          id="support-document-form"
          onSubmit={handleSubmit}
          style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
        >
          {/* Payment Code Section */}
          <div
            style={{
              paddingBottom: "1.25rem",
              borderBottom: "1px solid var(--color-border)",
              display: "flex",
              flexDirection: "column",
              gap: "1rem",
            }}
          >
            <h3 style={sectionHeadingStyle}>Detalles del Documento</h3>

            <Select
              label="Código de Pago *"
              value={formData.payment_code}
              error={errors.payment_code}
              onChange={(e) =>
                handleChange(
                  "payment_code",
                  e.target.value as SupportDocumentPaymentCode
                )
              }
            >
              <option value="" disabled>
                Selecciona un código de pago...
              </option>
              {PAYMENT_CODES.map((code) => (
                <option key={code.value} value={code.value}>
                  {code.label}
                </option>
              ))}
            </Select>
          </div>

          {/* Provider Section - Required */}
          <div
            style={{
              paddingBottom: "1.25rem",
              borderBottom: "1px solid var(--color-border)",
              display: "flex",
              flexDirection: "column",
              gap: "1rem",
            }}
          >
            <h3 style={sectionHeadingStyle}>
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
              }}
            >
              <Input
                label="Número de Documento *"
                type="text"
                value={formData.provider.id}
                onChange={(e) => handleChange("provider.id", e.target.value)}
                error={errors["provider.id"]}
                placeholder="Ingresa el NIT o CC del proveedor"
              />

              <Select
                label="Tipo de Documento *"
                value={formData.provider.document_type}
                onChange={(e) =>
                  handleChange(
                    "provider.document_type",
                    e.target.value as ProviderDocumentType
                  )
                }
              >
                {DOCUMENT_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </Select>
            </div>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: "1rem",
              }}
            >
              <Input
                label="Nombre *"
                type="text"
                value={formData.provider.name}
                onChange={(e) => handleChange("provider.name", e.target.value)}
                error={errors["provider.name"]}
                placeholder="Ingresa el nombre del proveedor"
              />

              <Input
                label="Correo Electrónico *"
                type="email"
                value={formData.provider.email}
                onChange={(e) => handleChange("provider.email", e.target.value)}
                error={errors["provider.email"]}
                placeholder="Ingresa el correo del proveedor"
              />
            </div>
          </div>

          {/* Items Section */}
          <div
            style={{
              paddingBottom: "1.25rem",
              borderBottom: "1px solid var(--color-border)",
              display: "flex",
              flexDirection: "column",
              gap: "1rem",
            }}
          >
            <h3 style={sectionHeadingStyle}>Artículos del Documento</h3>
            {errors.items && (
              <p
                style={{
                  margin: 0,
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.items}
              </p>
            )}
            {formData.items.length > 0 && (
              <Table>
                <thead>
                  <tr>
                    <th style={{ width: "110px" }} data-numeric>
                      Cantidad *
                    </th>
                    <th style={{ width: "160px" }} data-numeric>
                      Precio *
                    </th>
                    <th>Descripción *</th>
                    <th style={{ width: "90px" }}></th>
                  </tr>
                </thead>
                <tbody>
                  {formData.items.map((item, index) => (
                    <tr key={index}>
                      <td data-numeric>
                        <Input
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
                          error={errors[`items.${index}.quantity`]}
                          placeholder="0"
                        />
                      </td>
                      <td data-numeric>
                        <Input
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
                          error={errors[`items.${index}.price`]}
                          placeholder="0.00"
                        />
                      </td>
                      <td>
                        <Input
                          type="text"
                          value={item.description}
                          minLength={5}
                          onChange={(e) =>
                            updateItem(index, "description", e.target.value)
                          }
                          error={errors[`items.${index}.description`]}
                          placeholder="Ingresa la descripción del artículo"
                        />
                      </td>
                      <td>
                        <Button
                          type="button"
                          variant="danger"
                          size="sm"
                          onClick={() => removeItem(index)}
                        >
                          Eliminar
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            )}
            <Button
              type="button"
              variant="primary"
              fullWidth
              onClick={addItem}
            >
              + Agregar Artículo
            </Button>
          </div>

          {/* Summary */}
          <div
            style={{
              padding: "1.5rem",
              backgroundColor: "var(--color-surface-hover)",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3 style={sectionHeadingStyle}>Resumen</h3>
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
                    fontWeight: 700,
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
                    fontWeight: 700,
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
                    fontWeight: 500,
                  }}
                >
                  Total a Pagar
                </div>
                <div
                  style={{
                    fontSize: "1.5rem",
                    fontWeight: 700,
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
        </form>
      </Modal>

      {/* Confirmation Modal */}
      {showConfirmation && (
        <Modal
          open
          onClose={handleCancelConfirmation}
          title="Confirmar Documento Soporte"
          size="sm"
          footer={
            <>
              <Button
                variant="secondary"
                onClick={handleCancelConfirmation}
              >
                Cancelar
              </Button>
              <Button
                type="button"
                onClick={handleConfirmSubmit}
                disabled={isLoading}
              >
                {isLoading ? "Creando..." : "Confirmar"}
              </Button>
            </>
          }
        >
          <div style={{ textAlign: "center" }}>
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
                fontWeight: 700,
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
                margin: 0,
                fontSize: "0.95rem",
                color: "var(--color-text-secondary)",
              }}
            >
              Proveedor: <strong>{formData.provider.name}</strong> (
              {formData.provider.id})
            </p>
          </div>
        </Modal>
      )}
    </>
  );
}
