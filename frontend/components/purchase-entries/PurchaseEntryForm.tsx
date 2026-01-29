"use client";

import { useState, useEffect, useRef } from "react";
import { supplierCatalogApi } from "@/lib/api/supplierCatalog";
import type { Supplier } from "@/types/supplier";
import type { SupplierCatalogItem } from "@/types/supplierCatalog";
import type {
  CreatePurchaseEntryRequest,
  CreatePurchaseEntryItemRequest,
} from "@/types/purchaseEntry";

interface PurchaseEntryFormProps {
  suppliers: Supplier[];
  onSubmit: (
    data: CreatePurchaseEntryRequest,
    files: { pdf?: File | null; xml?: File | null; zip?: File | null },
  ) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

interface FormItem {
  product_id: string;
  quantity: string;
  unit_cost: string;
  total_cost: string;
  lastEdited: "unit_cost" | "total_cost";
}

export default function PurchaseEntryForm({
  suppliers,
  onSubmit,
  onCancel,
  isLoading = false,
}: PurchaseEntryFormProps) {
  const [formData, setFormData] = useState({
    supplier_id: "",
    invoice_reference: "",
    entry_date: new Date().toISOString().split("T")[0],
    notes: "",
  });

  const [items, setItems] = useState<FormItem[]>([
    { product_id: "", quantity: "", unit_cost: "", total_cost: "", lastEdited: "unit_cost" },
  ]);

  const [pdfFile, setPdfFile] = useState<File | null>(null);
  const [xmlFile, setXmlFile] = useState<File | null>(null);
  const [zipFile, setZipFile] = useState<File | null>(null);
  const pdfInputRef = useRef<HTMLInputElement>(null);
  const xmlInputRef = useRef<HTMLInputElement>(null);
  const zipInputRef = useRef<HTMLInputElement>(null);

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [catalogItems, setCatalogItems] = useState<SupplierCatalogItem[]>([]);
  const [loadingCatalog, setLoadingCatalog] = useState<boolean>(false);

  // Fetch supplier's catalog when supplier is selected
  useEffect(() => {
    const fetchCatalog = async () => {
      if (!formData.supplier_id) {
        setCatalogItems([]);
        // Reset items when supplier changes
        setItems([{ product_id: "", quantity: "", unit_cost: "", total_cost: "", lastEdited: "unit_cost" }]);
        return;
      }

      try {
        setLoadingCatalog(true);
        const catalog = await supplierCatalogApi.getProductsFromSupplier(
          formData.supplier_id,
        );
        setCatalogItems(catalog);
        // Reset items when supplier changes
        setItems([{ product_id: "", quantity: "", unit_cost: "", total_cost: "", lastEdited: "unit_cost" }]);
      } catch (err) {
        console.error("Error fetching supplier catalog:", err);
        setCatalogItems([]);
      } finally {
        setLoadingCatalog(false);
      }
    };

    fetchCatalog();
  }, [formData.supplier_id]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.supplier_id) {
      newErrors.supplier_id = "El proveedor es requerido";
    }

    // Validate items - check that unit_cost exists (either entered directly or calculated from total)
    const validItems = items.filter(
      (item) => item.product_id && item.quantity && item.unit_cost && parseFloat(item.unit_cost) > 0,
    );

    if (validItems.length === 0) {
      newErrors.items = "Debe agregar al menos un producto con cantidad y costo válidos";
    }

    // Check for invalid quantities or costs
    items.forEach((item, index) => {
      if (item.product_id) {
        if (!item.quantity || parseFloat(item.quantity) <= 0) {
          newErrors[`item_${index}_quantity`] = "Cantidad inválida";
        }
        // Unit cost must be positive (either entered directly or calculated from total)
        const unitCost = parseFloat(item.unit_cost) || 0;
        if (unitCost <= 0) {
          newErrors[`item_${index}_unit_cost`] = "Ingrese costo unitario o total";
        }
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

    const validItems: CreatePurchaseEntryItemRequest[] = items
      .filter((item) => item.product_id && item.quantity && item.unit_cost)
      .map((item) => ({
        product_id: item.product_id,
        quantity: item.quantity,
        unit_cost: item.unit_cost,
      }));

    const submitData: CreatePurchaseEntryRequest = {
      supplier_id: formData.supplier_id,
      items: validItems,
    };

    if (formData.invoice_reference.trim()) {
      submitData.invoice_reference = formData.invoice_reference.trim();
    }
    if (formData.entry_date) {
      submitData.entry_date = new Date(formData.entry_date).toISOString();
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
      if (
        file.type !== "text/xml" &&
        file.type !== "application/xml" &&
        !file.name.endsWith(".xml")
      ) {
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
      if (
        file.type !== "application/zip" &&
        file.type !== "application/x-zip-compressed" &&
        !file.name.toLowerCase().endsWith(".zip")
      ) {
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

  const handleAddItem = () => {
    setItems([...items, { product_id: "", quantity: "", unit_cost: "", total_cost: "", lastEdited: "unit_cost" }]);
  };

  const handleRemoveItem = (index: number) => {
    if (items.length > 1) {
      setItems(items.filter((_, i) => i !== index));
    }
  };

  const handleItemChange = (
    index: number,
    field: keyof FormItem,
    value: string,
  ) => {
    const newItems = [...items];
    newItems[index] = { ...newItems[index], [field]: value };
    setItems(newItems);

    // Clear item-specific errors
    const errorKey = `item_${index}_${field}`;
    if (errors[errorKey]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[errorKey];
        return newErrors;
      });
    }
    if (errors.items) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.items;
        return newErrors;
      });
    }
  };

  // Handle unit cost change - calculate total cost
  const handleUnitCostChange = (index: number, value: string) => {
    const newItems = [...items];
    const quantity = parseFloat(newItems[index].quantity) || 0;
    const unitCost = parseFloat(value) || 0;
    const totalCost = quantity * unitCost;

    newItems[index] = {
      ...newItems[index],
      unit_cost: value,
      total_cost: totalCost > 0 ? totalCost.toFixed(2) : "",
      lastEdited: "unit_cost",
    };
    setItems(newItems);

    // Clear errors
    clearItemErrors(index, "unit_cost");
  };

  // Handle total cost change - calculate unit cost with high precision
  const handleTotalCostChange = (index: number, value: string) => {
    const newItems = [...items];
    const quantity = parseFloat(newItems[index].quantity) || 0;
    const totalCost = parseFloat(value) || 0;
    
    // Calculate unit cost with high precision (6 decimals)
    const unitCost = quantity > 0 ? totalCost / quantity : 0;

    newItems[index] = {
      ...newItems[index],
      total_cost: value,
      unit_cost: unitCost > 0 ? unitCost.toFixed(6) : "",
      lastEdited: "total_cost",
    };
    setItems(newItems);

    // Clear errors
    clearItemErrors(index, "unit_cost");
  };

  // Helper to clear item-specific errors
  const clearItemErrors = (index: number, field: string) => {
    const errorKey = `item_${index}_${field}`;
    if (errors[errorKey] || errors.items) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[errorKey];
        delete newErrors.items;
        return newErrors;
      });
    }
  };

  // Handle quantity change - recalculate based on lastEdited field
  const handleQuantityChange = (index: number, value: string) => {
    const newItems = [...items];
    const item = newItems[index];
    const quantity = parseFloat(value) || 0;

    if (item.lastEdited === "total_cost" && item.total_cost) {
      // If user was entering totals, recalculate unit cost
      const totalCost = parseFloat(item.total_cost) || 0;
      const unitCost = quantity > 0 ? totalCost / quantity : 0;
      newItems[index] = {
        ...item,
        quantity: value,
        unit_cost: unitCost > 0 ? unitCost.toFixed(6) : "",
      };
    } else {
      // If user was entering unit costs (default), recalculate total
      const unitCost = parseFloat(item.unit_cost) || 0;
      const totalCost = quantity * unitCost;
      newItems[index] = {
        ...item,
        quantity: value,
        total_cost: totalCost > 0 ? totalCost.toFixed(2) : "",
      };
    }

    setItems(newItems);
    clearItemErrors(index, "quantity");
  };

  const getCatalogItemByProductId = (productId: string) =>
    catalogItems.find((item) => item.product_id === productId);

  const calculateItemTotal = (item: FormItem): number => {
    // Use total_cost if available, otherwise calculate from quantity * unit_cost
    if (item.total_cost) {
      return parseFloat(item.total_cost) || 0;
    }
    const qty = parseFloat(item.quantity) || 0;
    const cost = parseFloat(item.unit_cost) || 0;
    return qty * cost;
  };

  const calculateGrandTotal = (): number => {
    return items.reduce((sum, item) => sum + calculateItemTotal(item), 0);
  };

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
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

  // Get catalog items that haven't been selected yet
  const getAvailableCatalogItems = (currentIndex: number) => {
    const selectedProductIds = items
      .filter((_, i) => i !== currentIndex)
      .map((item) => item.product_id);
    return catalogItems.filter(
      (item) => !selectedProductIds.includes(item.product_id),
    );
  };

  // Handle product selection (unit_cost is entered manually for each purchase)
  const handleProductSelect = (index: number, productId: string) => {
    const newItems = [...items];
    newItems[index] = {
      ...newItems[index],
      product_id: productId,
    };
    setItems(newItems);

    // Clear item-specific errors
    if (errors.items) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors.items;
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
          maxWidth: "900px",
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
          Nueva Entrada de Compra
        </h2>

        <form onSubmit={handleSubmit}>
          {/* Supplier and Date Row */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "2fr 1fr",
              gap: "1rem",
              marginBottom: "1.5rem",
            }}
          >
            <div>
              <label style={labelStyle}>Proveedor *</label>
              <select
                value={formData.supplier_id}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    supplier_id: e.target.value,
                  }))
                }
                style={inputStyle(!!errors.supplier_id)}
              >
                <option value="">Seleccionar proveedor</option>
                {suppliers.map((supplier) => (
                  <option key={supplier.id} value={supplier.id}>
                    {supplier.name}
                  </option>
                ))}
              </select>
              {errors.supplier_id && (
                <p style={errorStyle}>{errors.supplier_id}</p>
              )}
            </div>

            <div>
              <label style={labelStyle}>Fecha de Entrada</label>
              <input
                type="date"
                value={formData.entry_date}
                onChange={(e) =>
                  setFormData((prev) => ({
                    ...prev,
                    entry_date: e.target.value,
                  }))
                }
                style={inputStyle(false)}
              />
            </div>
          </div>

          {/* Invoice Reference */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Referencia de Factura</label>
            <input
              type="text"
              value={formData.invoice_reference}
              onChange={(e) =>
                setFormData((prev) => ({
                  ...prev,
                  invoice_reference: e.target.value,
                }))
              }
              style={inputStyle(false)}
              placeholder="Número de factura del proveedor (opcional)"
              maxLength={255}
            />
          </div>

          {/* Items Section */}
          <div
            style={{
              marginBottom: "1.5rem",
              padding: "1.5rem",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-sm)",
              border: `1px solid ${errors.items ? "var(--color-danger)" : "var(--color-border)"}`,
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
              <h3
                style={{
                  margin: 0,
                  fontSize: "1rem",
                  fontWeight: "600",
                  color: "var(--color-text-primary)",
                }}
              >
                Productos *
              </h3>
              <button
                type="button"
                onClick={handleAddItem}
                style={{
                  padding: "0.5rem 1rem",
                  backgroundColor: "var(--color-primary)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: "0.875rem",
                }}
              >
                + Agregar Producto
              </button>
            </div>

            {errors.items && <p style={errorStyle}>{errors.items}</p>}

            {/* Items Table */}
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse" }}>
                <thead>
                  <tr>
                    <th
                      style={{
                        textAlign: "left",
                        padding: "0.5rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        borderBottom: "1px solid var(--color-border)",
                        minWidth: "250px",
                      }}
                    >
                      Producto
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.5rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        borderBottom: "1px solid var(--color-border)",
                        width: "120px",
                      }}
                    >
                      Cantidad
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.5rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        borderBottom: "1px solid var(--color-border)",
                        width: "150px",
                      }}
                    >
                      Costo Unit.
                    </th>
                    <th
                      style={{
                        textAlign: "right",
                        padding: "0.5rem",
                        color: "var(--color-text-secondary)",
                        fontSize: "0.875rem",
                        borderBottom: "1px solid var(--color-border)",
                        width: "150px",
                      }}
                    >
                      Total
                    </th>
                    <th
                      style={{
                        padding: "0.5rem",
                        borderBottom: "1px solid var(--color-border)",
                        width: "50px",
                      }}
                    />
                  </tr>
                </thead>
                <tbody>
                  {items.map((item, index) => {
                    const selectedCatalogItem = getCatalogItemByProductId(
                      item.product_id,
                    );
                    return (
                      <tr key={index}>
                        <td style={{ padding: "0.5rem" }}>
                          <select
                            value={item.product_id}
                            onChange={(e) =>
                              handleProductSelect(index, e.target.value)
                            }
                            disabled={!formData.supplier_id || loadingCatalog}
                            style={{
                              width: "100%",
                              padding: "0.5rem",
                              border: "1px solid var(--color-border)",
                              borderRadius: "var(--radius-sm)",
                              backgroundColor: "var(--color-surface)",
                              color: "var(--color-text-primary)",
                              fontSize: "0.875rem",
                              opacity:
                                !formData.supplier_id || loadingCatalog
                                  ? 0.6
                                  : 1,
                            }}
                          >
                            <option value="">
                              {!formData.supplier_id
                                ? "Primero seleccione un proveedor"
                                : loadingCatalog
                                  ? "Cargando productos..."
                                  : catalogItems.length === 0
                                    ? "El proveedor no tiene productos"
                                    : "Seleccionar producto"}
                            </option>
                            {getAvailableCatalogItems(index).map(
                              (catalogItem) => (
                                <option
                                  key={catalogItem.product_id}
                                  value={catalogItem.product_id}
                                >
                                  {catalogItem.product_name} -{" "}
                                  {formatCurrency(
                                    parseFloat(catalogItem.unit_cost),
                                  )}
                                </option>
                              ),
                            )}
                            {/* Keep selected product in options even if selected elsewhere */}
                            {selectedCatalogItem &&
                              !getAvailableCatalogItems(index).find(
                                (c) =>
                                  c.product_id ===
                                  selectedCatalogItem.product_id,
                              ) && (
                                <option value={selectedCatalogItem.product_id}>
                                  {selectedCatalogItem.product_name} -{" "}
                                  {formatCurrency(
                                    parseFloat(selectedCatalogItem.unit_cost),
                                  )}
                                </option>
                              )}
                          </select>
                        </td>
                        <td style={{ padding: "0.5rem" }}>
                          <input
                            type="number"
                            value={item.quantity}
                            onChange={(e) =>
                              handleQuantityChange(index, e.target.value)
                            }
                            placeholder="0"
                            min="0.01"
                            step="0.01"
                            style={{
                              width: "100%",
                              padding: "0.5rem",
                              border: `1px solid ${
                                errors[`item_${index}_quantity`]
                                  ? "var(--color-danger)"
                                  : "var(--color-border)"
                              }`,
                              borderRadius: "var(--radius-sm)",
                              backgroundColor: "var(--color-surface)",
                              color: "var(--color-text-primary)",
                              fontSize: "0.875rem",
                              textAlign: "right",
                            }}
                          />
                        </td>
                        <td style={{ padding: "0.5rem" }}>
                          <input
                            type="number"
                            value={item.unit_cost}
                            onChange={(e) =>
                              handleUnitCostChange(index, e.target.value)
                            }
                            placeholder="0"
                            min="0"
                            step="0.000001"
                            style={{
                              width: "100%",
                              padding: "0.5rem",
                              border: `1px solid ${
                                errors[`item_${index}_unit_cost`]
                                  ? "var(--color-danger)"
                                  : "var(--color-border)"
                              }`,
                              borderRadius: "var(--radius-sm)",
                              backgroundColor: "var(--color-surface)",
                              color: "var(--color-text-primary)",
                              fontSize: "0.875rem",
                              textAlign: "right",
                            }}
                          />
                        </td>
                        <td style={{ padding: "0.5rem" }}>
                          <input
                            type="number"
                            value={item.total_cost}
                            onChange={(e) =>
                              handleTotalCostChange(index, e.target.value)
                            }
                            placeholder="0"
                            min="0"
                            step="0.01"
                            style={{
                              width: "100%",
                              padding: "0.5rem",
                              border: "1px solid var(--color-border)",
                              borderRadius: "var(--radius-sm)",
                              backgroundColor: "var(--color-surface)",
                              color: "var(--color-text-primary)",
                              fontSize: "0.875rem",
                              textAlign: "right",
                              fontWeight: "600",
                            }}
                          />
                        </td>
                        <td style={{ padding: "0.5rem", textAlign: "center" }}>
                          {items.length > 1 && (
                            <button
                              type="button"
                              onClick={() => handleRemoveItem(index)}
                              style={{
                                padding: "0.25rem 0.5rem",
                                backgroundColor: "var(--color-danger)",
                                color: "white",
                                border: "none",
                                borderRadius: "var(--radius-sm)",
                                cursor: "pointer",
                                fontSize: "0.75rem",
                              }}
                            >
                              ×
                            </button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
                <tfoot>
                  <tr>
                    <td
                      colSpan={3}
                      style={{
                        padding: "0.75rem 0.5rem",
                        textAlign: "right",
                        fontWeight: "600",
                        borderTop: "2px solid var(--color-border)",
                      }}
                    >
                      Total General:
                    </td>
                    <td
                      style={{
                        padding: "0.75rem 0.5rem",
                        textAlign: "right",
                        fontWeight: "bold",
                        fontSize: "1.1rem",
                        color: "var(--color-success)",
                        borderTop: "2px solid var(--color-border)",
                      }}
                    >
                      {formatCurrency(calculateGrandTotal())}
                    </td>
                    <td
                      style={{ borderTop: "2px solid var(--color-border)" }}
                    />
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>

          {/* Notes */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={labelStyle}>Notas</label>
            <textarea
              value={formData.notes}
              onChange={(e) =>
                setFormData((prev) => ({ ...prev, notes: e.target.value }))
              }
              style={{
                ...inputStyle(false),
                minHeight: "80px",
                resize: "vertical",
              }}
              placeholder="Notas adicionales sobre esta entrada (opcional)"
              maxLength={1000}
            />
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

            {/* PDF Upload */}
            <div style={{ marginBottom: "1rem" }}>
              <label style={{ ...labelStyle, fontSize: "0.875rem" }}>
                Factura PDF
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
              <label style={{ ...labelStyle, fontSize: "0.875rem" }}>
                Factura Electrónica XML
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
              Los documentos se subirán después de guardar la entrada.
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
              {isLoading ? "Guardando..." : "Registrar Entrada"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
