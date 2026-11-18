"use client";

import { useState, useEffect, useRef } from "react";
import type { ExpenseCategory, Expense, CreateExpenseRequest } from "@/types/expense";
import type { Supplier } from "@/types/supplier";
import { Input, Select, Textarea, Button } from "@/components/ui";

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

  const [pdfPreviewUrl, setPdfPreviewUrl] = useState<string | null>(null);
  const [isExtractingPdf, setIsExtractingPdf] = useState(false);
  const [windowWidth, setWindowWidth] = useState(
    typeof window !== "undefined" ? window.innerWidth : 1200
  );

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Track window width for responsive layout
  useEffect(() => {
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // Custom modal shell: lock body scroll + close on Escape
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [onCancel]);

  // Cleanup object URL on unmount
  useEffect(() => {
    return () => {
      if (pdfPreviewUrl) {
        URL.revokeObjectURL(pdfPreviewUrl);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
      extractPdfFromZip(file);
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
    if (pdfPreviewUrl) {
      URL.revokeObjectURL(pdfPreviewUrl);
      setPdfPreviewUrl(null);
    }
    setErrors((prev) => {
      const newErrors = { ...prev };
      delete newErrors.zipPreview;
      return newErrors;
    });
  };

  const extractPdfFromZip = async (file: File) => {
    setIsExtractingPdf(true);
    try {
      const JSZip = (await import("jszip")).default;
      const zip = await JSZip.loadAsync(file);

      const pdfEntry = Object.values(zip.files).find(
        (entry) => !entry.dir && entry.name.toLowerCase().endsWith(".pdf")
      );

      if (!pdfEntry) {
        setErrors((prev) => ({
          ...prev,
          zipPreview: "El ZIP no contiene un archivo PDF para previsualizar",
        }));
        return;
      }

      const blob = await pdfEntry.async("blob");
      const url = URL.createObjectURL(
        new Blob([blob], { type: "application/pdf" })
      );

      if (pdfPreviewUrl) {
        URL.revokeObjectURL(pdfPreviewUrl);
      }

      setPdfPreviewUrl(url);
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.zipPreview;
        return newErrors;
      });
    } catch (err) {
      console.error("Error extracting PDF from ZIP:", err);
      setErrors((prev) => ({
        ...prev,
        zipPreview: "No se pudo extraer el PDF del archivo ZIP",
      }));
    } finally {
      setIsExtractingPdf(false);
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

  // Filter active categories
  const activeCategories = categories.filter((cat) => cat.is_active);

  const showSidePreview = pdfPreviewUrl && windowWidth >= 1024;
  const showInlinePreview = pdfPreviewUrl && windowWidth < 1024;

  const pdfPreview = pdfPreviewUrl ? (
    <div
      style={{
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-sm)",
        overflow: "hidden",
        backgroundColor: "var(--color-surface)",
      }}
    >
      <div
        style={{
          padding: "0.5rem 0.75rem",
          backgroundColor: "var(--color-bg)",
          borderBottom: "1px solid var(--color-border)",
          fontSize: "0.75rem",
          fontWeight: "600",
          color: "var(--color-text-muted)",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <span>Vista previa de factura</span>
        <button
          type="button"
          onClick={() => window.open(pdfPreviewUrl, "_blank")}
          style={{
            padding: "0.25rem 0.5rem",
            backgroundColor: "transparent",
            color: "var(--color-primary)",
            border: "1px solid var(--color-primary)",
            borderRadius: "var(--radius-sm)",
            cursor: "pointer",
            fontSize: "0.7rem",
          }}
        >
          Abrir en nueva pestaña
        </button>
      </div>
      <iframe
        src={pdfPreviewUrl}
        style={{
          width: "100%",
          height: showSidePreview ? "calc(90vh - 120px)" : "400px",
          border: "none",
          display: "block",
        }}
        title="Vista previa del PDF"
      />
    </div>
  ) : null;

  return (
    <div
      role="presentation"
      onClick={onCancel}
      style={{
        position: "fixed",
        inset: 0,
        background: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "24px",
        zIndex: 1000,
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        style={{
          background: "var(--color-surface)",
          border: "1px solid var(--color-border)",
          borderRadius: "var(--radius-lg)",
          boxShadow: "var(--shadow-xl)",
          width: "100%",
          maxWidth: showSidePreview ? "min(1400px, 95vw)" : "780px",
          maxHeight: "90vh",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: "18px 22px",
            borderBottom: "1px solid var(--color-border)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            gap: "12px",
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: "17px",
              fontWeight: 700,
              color: "var(--color-text-primary)",
            }}
          >
            {expense ? "Editar Gasto" : "Nuevo Gasto"}
          </h2>
          <button
            type="button"
            onClick={onCancel}
            aria-label="Cerrar"
            style={{
              border: "none",
              background: "transparent",
              color: "var(--color-text-muted)",
              fontSize: "16px",
              lineHeight: 1,
              width: "32px",
              height: "32px",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
            }}
          >
            ✕
          </button>
        </div>

        {/* Body: form (left) + optional side PDF preview (right on wide screens) */}
        <div style={{ display: "flex", flex: 1, minHeight: 0, overflow: "hidden" }}>
          <form
            id="expense-form"
            onSubmit={handleSubmit}
            style={{
              display: "flex",
              flexDirection: "column",
              gap: "1.25rem",
              flex: showSidePreview ? "0 0 520px" : 1,
              minWidth: 0,
              overflowY: "auto",
              padding: "22px",
            }}
          >
        {/* ZIP Upload — Step 1 */}
        <div
          style={{
            padding: "1rem",
            backgroundColor: "var(--color-bg)",
            borderRadius: "var(--radius-sm)",
            border: `1px solid ${zipFile ? "var(--color-primary)" : "var(--color-border)"}`,
          }}
        >
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: "500",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Paso 1: Sube el archivo ZIP con factura (PDF + XML)
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
              <Button variant="danger" size="sm" onClick={handleRemoveZip}>
                Quitar
              </Button>
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
                opacity: pdfFile || xmlFile ? 0.5 : 1,
              }}
            />
          )}
          {errors.zip && (
            <p
              style={{
                margin: "0.25rem 0 0 0",
                color: "var(--color-danger)",
                fontSize: "0.875rem",
              }}
            >
              {errors.zip}
            </p>
          )}
          {isExtractingPdf && (
            <p
              style={{
                margin: "0.5rem 0 0 0",
                fontSize: "0.75rem",
                color: "var(--color-text-muted)",
              }}
            >
              Extrayendo PDF para previsualización...
            </p>
          )}
          {errors.zipPreview && (
            <p
              style={{
                margin: "0.25rem 0 0 0",
                color: "var(--color-warning)",
                fontSize: "0.75rem",
              }}
            >
              {errors.zipPreview}
            </p>
          )}
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

        {/* PDF preview (stacked inline on narrow screens; side panel on wide) */}
        {showInlinePreview && <div>{pdfPreview}</div>}

        {/* Category and Date Row */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "2fr 1fr",
            gap: "1rem",
          }}
        >
          <Select
            label="Categoría *"
            value={formData.category_id}
            onChange={(e) => handleChange("category_id", e.target.value)}
            error={errors.category_id}
          >
            <option value="">Seleccionar categoría</option>
            {activeCategories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>

          <Input
            label="Fecha del Gasto"
            type="date"
            value={formData.expense_date}
            onChange={(e) => handleChange("expense_date", e.target.value)}
          />
        </div>

        {/* Amount */}
        <Input
          label="Monto *"
          type="number"
          value={formData.amount}
          onChange={(e) => handleChange("amount", e.target.value)}
          error={errors.amount}
          placeholder="0"
          min="0.01"
          step="0.01"
        />

        {/* Description */}
        <div>
          <Input
            label="Descripción *"
            type="text"
            value={formData.description}
            onChange={(e) => handleChange("description", e.target.value)}
            error={errors.description}
            placeholder="Descripción del gasto"
            maxLength={500}
          />
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
        <Select
          label="Proveedor (opcional)"
          value={formData.supplier_id}
          onChange={(e) => handleChange("supplier_id", e.target.value)}
        >
          <option value="">Sin proveedor</option>
          {suppliers.map((supplier) => (
            <option key={supplier.id} value={supplier.id}>
              {supplier.name}
            </option>
          ))}
        </Select>

        {/* Reference */}
        <Input
          label="Referencia (opcional)"
          type="text"
          value={formData.reference}
          onChange={(e) => handleChange("reference", e.target.value)}
          error={errors.reference}
          placeholder="Número de factura o recibo"
          maxLength={255}
        />

        {/* Notes */}
        <div>
          <Textarea
            label="Notas (opcional)"
            value={formData.notes}
            onChange={(e) => handleChange("notes", e.target.value)}
            error={errors.notes}
            placeholder="Notas adicionales sobre este gasto"
            maxLength={1000}
          />
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
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
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
                <Button variant="danger" size="sm" onClick={handleRemovePdf}>
                  Quitar
                </Button>
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
            {errors.pdf && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.pdf}
              </p>
            )}
          </div>

          {/* XML Upload */}
          <div>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                fontSize: "0.875rem",
                color: "var(--color-text-primary)",
              }}
            >
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
                <Button variant="danger" size="sm" onClick={handleRemoveXml}>
                  Quitar
                </Button>
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
            {errors.xml && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "var(--color-danger)",
                  fontSize: "0.875rem",
                }}
              >
                {errors.xml}
              </p>
            )}
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
          </form>

          {showSidePreview && (
            <div
              style={{
                flex: 1,
                minWidth: 0,
                borderLeft: "1px solid var(--color-border)",
                padding: "22px",
                overflowY: "auto",
                background: "var(--color-bg)",
              }}
            >
              {pdfPreview}
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: "16px 22px",
            borderTop: "1px solid var(--color-border)",
            background: "var(--color-bg)",
            display: "flex",
            gap: "12px",
            justifyContent: "flex-end",
          }}
        >
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="expense-form" disabled={isLoading}>
            {isLoading
              ? "Guardando..."
              : expense
              ? "Actualizar Gasto"
              : "Registrar Gasto"}
          </Button>
        </div>
      </div>
    </div>
  );
}
