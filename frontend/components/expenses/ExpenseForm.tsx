"use client";

import { useState, useEffect, useRef } from "react";
import type { ExpenseCategory, Expense, CreateExpenseRequest } from "@/types/expense";
import type { Supplier } from "@/types/supplier";

interface ExpenseFormProps {
  categories: ExpenseCategory[];
  suppliers: Supplier[];
  expense?: Expense | null;
  onSubmit: (data: CreateExpenseRequest, files: { pdf?: File | null; xml?: File | null; zip?: File | null }) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export default function ExpenseForm({
  categories,
  suppliers,
  expense,
  onSubmit,
  onCancel,
  isLoading = false,
}: ExpenseFormProps) {
  const [formData, setFormData] = useState({
    category_id: "",
    supplier_id: "",
    amount: "",
    description: "",
    expense_date: new Date().toISOString().split("T")[0],
    reference: "",
    notes: "",
  });

  const [pdfFile, setPdfFile] = useState<File | null>(null);
  const [xmlFile, setXmlFile] = useState<File | null>(null);
  const [zipFile, setZipFile] = useState<File | null>(null);
  const pdfInputRef = useRef<HTMLInputElement>(null);
  const xmlInputRef = useRef<HTMLInputElement>(null);
  const zipInputRef = useRef<HTMLInputElement>(null);

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Populate form when editing
  useEffect(() => {
    if (expense) {
      setFormData({
        category_id: expense.category_id || "",
        supplier_id: expense.supplier_id || "",
        amount: expense.amount || "",
        description: expense.description || "",
        expense_date: expense.expense_date
          ? new Date(expense.expense_date).toISOString().split("T")[0]
          : new Date().toISOString().split("T")[0],
        reference: expense.reference || "",
        notes: expense.notes || "",
      });
    }
  }, [expense]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.category_id) {
      newErrors.category_id = "La categoría es requerida";
    }

    if (!formData.amount || parseFloat(formData.amount) <= 0) {
      newErrors.amount = "El monto debe ser mayor a 0";
    }

    if (!formData.description.trim()) {
      newErrors.description = "La descripción es requerida";
    }

    if (formData.description.length > 500) {
      newErrors.description = "La descripción no puede exceder 500 caracteres";
    }

    if (formData.reference && formData.reference.length > 255) {
      newErrors.reference = "La referencia no puede exceder 255 caracteres";
    }

    if (formData.notes && formData.notes.length > 1000) {
      newErrors.notes = "Las notas no pueden exceder 1000 caracteres";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    const submitData: CreateExpenseRequest = {
      category_id: formData.category_id,
      amount: formData.amount,
      description: formData.description.trim(),
    };

    if (formData.supplier_id) {
      submitData.supplier_id = formData.supplier_id;
    }
    if (formData.expense_date) {
      submitData.expense_date = new Date(formData.expense_date).toISOString();
    }
    if (formData.reference.trim()) {
      submitData.reference = formData.reference.trim();
    }
    if (formData.notes.trim()) {
      submitData.notes = formData.notes.trim();
    }

    await onSubmit(submitData, { pdf: pdfFile, xml: xmlFile, zip: zipFile });
  };

  const handlePdfChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      if (file.type !== "application/pdf") {
        setErrors((prev) => ({ ...prev, pdf: "El archivo debe ser PDF" }));
        return;
      }
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.pdf;
        return newErrors;
      });
      setPdfFile(file);
    }
  };

  const handleXmlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      if (file.type !== "text/xml" && file.type !== "application/xml" && !file.name.endsWith(".xml")) {
        setErrors((prev) => ({ ...prev, xml: "El archivo debe ser XML" }));
        return;
      }
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.xml;
        return newErrors;
      });
      setXmlFile(file);
    }
  };

  const handleRemovePdf = () => {
    setPdfFile(null);
    if (pdfInputRef.current) {
      pdfInputRef.current.value = "";
    }
  };

  const handleRemoveXml = () => {
    setXmlFile(null);
    if (xmlInputRef.current) {
      xmlInputRef.current.value = "";
    }
  };

  const handleZipChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      if (file.type !== "application/zip" && file.type !== "application/x-zip-compressed" && !file.name.toLowerCase().endsWith(".zip")) {
        setErrors((prev) => ({ ...prev, zip: "El archivo debe ser ZIP" }));
        return;
      }
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.zip;
        return newErrors;
      });
      setZipFile(file);
      // Clear individual PDF and XML files when ZIP is selected
      setPdfFile(null);
      setXmlFile(null);
      if (pdfInputRef.current) pdfInputRef.current.value = "";
      if (xmlInputRef.current) xmlInputRef.current.value = "";
    }
  };

  const handleRemoveZip = () => {
    setZipFile(null);
    if (zipInputRef.current) {
      zipInputRef.current.value = "";
    }
  };

  const handleChange = (
    field: string,
    value: string
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error when user types
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

  // Filter active categories
  const activeCategories = categories.filter((cat) => cat.is_active);

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
          {expense ? "Editar Gasto" : "Nuevo Gasto"}
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Category and Date Row */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "2fr 1fr",
              gap: "1rem",
              marginBottom: "1.5rem",
            }}
          >
            <div>
              <label style={labelStyle}>Categoría *</label>
              <select
                value={formData.category_id}
                onChange={(e) => handleChange("category_id", e.target.value)}
                style={inputStyle(!!errors.category_id)}
              >
                <option value="">Seleccionar categoría</option>
                {activeCategories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </select>
              {errors.category_id && <p style={errorStyle}>{errors.category_id}</p>}
            </div>

            <div>
              <label style={labelStyle}>Fecha del Gasto</label>
              <input
                type="date"
                value={formData.expense_date}
                onChange={(e) => handleChange("expense_date", e.target.value)}
                style={inputStyle(false)}
              />
            </div>
          </div>

          {/* Amount */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Monto *</label>
            <input
              type="number"
              value={formData.amount}
              onChange={(e) => handleChange("amount", e.target.value)}
              style={inputStyle(!!errors.amount)}
              placeholder="0"
              min="0.01"
              step="0.01"
            />
            {errors.amount && <p style={errorStyle}>{errors.amount}</p>}
          </div>

          {/* Description */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Descripción *</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => handleChange("description", e.target.value)}
              style={inputStyle(!!errors.description)}
              placeholder="Descripción del gasto"
              maxLength={500}
            />
            {errors.description && <p style={errorStyle}>{errors.description}</p>}
            <p
              style={{
                margin: "0.25rem 0 0 0",
                fontSize: "0.75rem",
                color: "var(--color-text-muted)",
                textAlign: "right",
              }}
            >
              {formData.description.length}/500
            </p>
          </div>

          {/* Supplier */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Proveedor (opcional)</label>
            <select
              value={formData.supplier_id}
              onChange={(e) => handleChange("supplier_id", e.target.value)}
              style={inputStyle(false)}
            >
              <option value="">Sin proveedor</option>
              {suppliers.map((supplier) => (
                <option key={supplier.id} value={supplier.id}>
                  {supplier.name}
                </option>
              ))}
            </select>
          </div>

          {/* Reference */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Referencia (opcional)</label>
            <input
              type="text"
              value={formData.reference}
              onChange={(e) => handleChange("reference", e.target.value)}
              style={inputStyle(!!errors.reference)}
              placeholder="Número de factura o recibo"
              maxLength={255}
            />
            {errors.reference && <p style={errorStyle}>{errors.reference}</p>}
          </div>

          {/* Notes */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Notas (opcional)</label>
            <textarea
              value={formData.notes}
              onChange={(e) => handleChange("notes", e.target.value)}
              style={{
                ...inputStyle(!!errors.notes),
                minHeight: "80px",
                resize: "vertical",
              }}
              placeholder="Notas adicionales sobre este gasto"
              maxLength={1000}
            />
            {errors.notes && <p style={errorStyle}>{errors.notes}</p>}
            <p
              style={{
                margin: "0.25rem 0 0 0",
                fontSize: "0.75rem",
                color: "var(--color-text-muted)",
                textAlign: "right",
              }}
            >
              {formData.notes.length}/1000
            </p>
          </div>

          {/* Documents Section */}
          <div
            style={{
              marginBottom: "1.5rem",
              padding: "1.5rem",
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
                color: "var(--color-text-primary)",
              }}
            >
              Documentos de Soporte (opcional)
            </h3>

            {/* Existing documents (when editing) */}
            {expense && (expense.pdf_storage_path || expense.xml_storage_path) && (
              <div
                style={{
                  marginBottom: "1rem",
                  padding: "0.75rem",
                  backgroundColor: "var(--color-surface)",
                  borderRadius: "var(--radius-sm)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <p
                  style={{
                    margin: "0 0 0.5rem 0",
                    fontSize: "0.875rem",
                    color: "var(--color-text-muted)",
                  }}
                >
                  Documentos actuales:
                </p>
                <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
                  {expense.pdf_storage_path && (
                    <span
                      style={{
                        padding: "0.25rem 0.75rem",
                        backgroundColor: "var(--color-danger-light)",
                        color: "var(--color-danger)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.75rem",
                        fontWeight: "600",
                      }}
                    >
                      PDF adjunto
                    </span>
                  )}
                  {expense.xml_storage_path && (
                    <span
                      style={{
                        padding: "0.25rem 0.75rem",
                        backgroundColor: "var(--color-success-light)",
                        color: "var(--color-success)",
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.75rem",
                        fontWeight: "600",
                      }}
                    >
                      XML adjunto
                    </span>
                  )}
                </div>
              </div>
            )}

            {/* PDF Upload */}
            <div style={{ marginBottom: "1rem" }}>
              <label style={{ ...labelStyle, fontSize: "0.875rem" }}>
                Documento PDF (factura, recibo)
              </label>
              {pdfFile ? (
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.75rem",
                    padding: "0.75rem",
                    backgroundColor: "var(--color-surface)",
                    borderRadius: "var(--radius-sm)",
                    border: "1px solid var(--color-border)",
                  }}
                >
                  <span
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "var(--color-danger-light)",
                      color: "var(--color-danger)",
                      borderRadius: "var(--radius-sm)",
                      fontSize: "0.75rem",
                      fontWeight: "600",
                    }}
                  >
                    PDF
                  </span>
                  <span
                    style={{
                      flex: 1,
                      fontSize: "0.875rem",
                      color: "var(--color-text-primary)",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {pdfFile.name}
                  </span>
                  <button
                    type="button"
                    onClick={handleRemovePdf}
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "transparent",
                      color: "var(--color-danger)",
                      border: "1px solid var(--color-danger)",
                      borderRadius: "var(--radius-sm)",
                      cursor: "pointer",
                      fontSize: "0.75rem",
                    }}
                  >
                    Quitar
                  </button>
                </div>
              ) : (
                <input
                  ref={pdfInputRef}
                  type="file"
                  accept=".pdf,application/pdf"
                  onChange={handlePdfChange}
                  disabled={!!zipFile}
                  style={{
                    width: "100%",
                    padding: "0.5rem",
                    border: `1px solid ${errors.pdf ? "var(--color-danger)" : "var(--color-border)"}`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.875rem",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                    opacity: zipFile ? 0.5 : 1,
                  }}
                />
              )}
              {errors.pdf && <p style={errorStyle}>{errors.pdf}</p>}
            </div>

            {/* XML Upload */}
            <div>
              <label style={{ ...labelStyle, fontSize: "0.875rem" }}>
                Documento XML (factura electrónica)
              </label>
              {xmlFile ? (
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.75rem",
                    padding: "0.75rem",
                    backgroundColor: "var(--color-surface)",
                    borderRadius: "var(--radius-sm)",
                    border: "1px solid var(--color-border)",
                  }}
                >
                  <span
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "var(--color-success-light)",
                      color: "var(--color-success)",
                      borderRadius: "var(--radius-sm)",
                      fontSize: "0.75rem",
                      fontWeight: "600",
                    }}
                  >
                    XML
                  </span>
                  <span
                    style={{
                      flex: 1,
                      fontSize: "0.875rem",
                      color: "var(--color-text-primary)",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {xmlFile.name}
                  </span>
                  <button
                    type="button"
                    onClick={handleRemoveXml}
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "transparent",
                      color: "var(--color-danger)",
                      border: "1px solid var(--color-danger)",
                      borderRadius: "var(--radius-sm)",
                      cursor: "pointer",
                      fontSize: "0.75rem",
                    }}
                  >
                    Quitar
                  </button>
                </div>
              ) : (
                <input
                  ref={xmlInputRef}
                  type="file"
                  accept=".xml,text/xml,application/xml"
                  onChange={handleXmlChange}
                  disabled={!!zipFile}
                  style={{
                    width: "100%",
                    padding: "0.5rem",
                    border: `1px solid ${errors.xml ? "var(--color-danger)" : "var(--color-border)"}`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.875rem",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                    opacity: zipFile ? 0.5 : 1,
                  }}
                />
              )}
              {errors.xml && <p style={errorStyle}>{errors.xml}</p>}
            </div>

            {/* Separator */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "1rem",
                margin: "1rem 0",
              }}
            >
              <div
                style={{
                  flex: 1,
                  height: "1px",
                  backgroundColor: "var(--color-border)",
                }}
              />
              <span
                style={{
                  fontSize: "0.75rem",
                  color: "var(--color-text-muted)",
                  fontWeight: "500",
                }}
              >
                O
              </span>
              <div
                style={{
                  flex: 1,
                  height: "1px",
                  backgroundColor: "var(--color-border)",
                }}
              />
            </div>

            {/* ZIP Upload */}
            <div>
              <label style={{ ...labelStyle, fontSize: "0.875rem" }}>
                Archivo ZIP (contiene PDF y XML)
              </label>
              {zipFile ? (
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.75rem",
                    padding: "0.75rem",
                    backgroundColor: "var(--color-surface)",
                    borderRadius: "var(--radius-sm)",
                    border: "1px solid var(--color-primary)",
                  }}
                >
                  <span
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "var(--color-primary-light)",
                      color: "var(--color-primary)",
                      borderRadius: "var(--radius-sm)",
                      fontSize: "0.75rem",
                      fontWeight: "600",
                    }}
                  >
                    ZIP
                  </span>
                  <span
                    style={{
                      flex: 1,
                      fontSize: "0.875rem",
                      color: "var(--color-text-primary)",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {zipFile.name}
                  </span>
                  <button
                    type="button"
                    onClick={handleRemoveZip}
                    style={{
                      padding: "0.25rem 0.5rem",
                      backgroundColor: "transparent",
                      color: "var(--color-danger)",
                      border: "1px solid var(--color-danger)",
                      borderRadius: "var(--radius-sm)",
                      cursor: "pointer",
                      fontSize: "0.75rem",
                    }}
                  >
                    Quitar
                  </button>
                </div>
              ) : (
                <input
                  ref={zipInputRef}
                  type="file"
                  accept=".zip,application/zip,application/x-zip-compressed"
                  onChange={handleZipChange}
                  disabled={!!(pdfFile || xmlFile)}
                  style={{
                    width: "100%",
                    padding: "0.5rem",
                    border: `1px solid ${errors.zip ? "var(--color-danger)" : "var(--color-border)"}`,
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.875rem",
                    backgroundColor: "var(--color-surface)",
                    color: "var(--color-text-primary)",
                    opacity: (pdfFile || xmlFile) ? 0.5 : 1,
                  }}
                />
              )}
              {errors.zip && <p style={errorStyle}>{errors.zip}</p>}
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  fontSize: "0.75rem",
                  color: "var(--color-text-muted)",
                }}
              >
                El ZIP debe contener exactamente 1 PDF y 1 XML.
              </p>
            </div>

            <p
              style={{
                margin: "0.75rem 0 0 0",
                fontSize: "0.75rem",
                color: "var(--color-text-muted)",
              }}
            >
              Los documentos se subirán después de guardar el gasto.
            </p>
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
              {isLoading ? "Guardando..." : expense ? "Actualizar Gasto" : "Registrar Gasto"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
