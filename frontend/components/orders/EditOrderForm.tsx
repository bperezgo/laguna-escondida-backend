"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import { productsApi } from "@/lib/api/products";
import { updateOrder } from "@/lib/api/orders";
import { generateInvoicePrintHTML } from "@/lib/templates/invoicePrint";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import type { Product } from "@/types/product";
import type { OpenBillWithProducts, OrderProductItem } from "@/types/order";
import type { OpenBillProductStatus } from "@/types/commandItem";

interface EditOrderFormProps {
  openBill: OpenBillWithProducts;
  onClose: () => void;
  onSuccess: () => void;
  onPayClick?: () => void;
  onRemoveClick?: () => void;
}

interface ProductWithQuantity {
  lineItemId: string;
  openBillProductId: string;
  product: Product;
  quantity: number;
  notes: string;
  status?: OpenBillProductStatus;
}

const STATUS_CONFIG: Record<
  OpenBillProductStatus,
  { label: string; bgColor: string; textColor: string }
> = {
  created: {
    label: "Creado",
    bgColor: "var(--color-surface-hover)",
    textColor: "var(--color-text-muted)",
  },
  in_progress: {
    label: "En Progreso",
    bgColor: "#dbeafe",
    textColor: "#1d4ed8",
  },
  completed: {
    label: "Completado",
    bgColor: "var(--color-success-light)",
    textColor: "var(--color-success)",
  },
  cancelled: {
    label: "Cancelado",
    bgColor: "var(--color-danger-light)",
    textColor: "var(--color-danger)",
  },
};

export default function EditOrderForm({
  openBill,
  onClose,
  onSuccess,
  onPayClick,
  onRemoveClick,
}: EditOrderFormProps) {
  const printRef = useRef<HTMLDivElement>(null);
  const [descriptor, setDescriptor] = useState(openBill.descriptor || "");
  const [temporalIdentifier, setTemporalIdentifier] = useState(
    openBill.temporal_identifier,
  );
  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [products, setProducts] = useState<Product[]>([]);
  const [selectedProducts, setSelectedProducts] = useState<
    Map<string, ProductWithQuantity>
  >(new Map());
  const [searchQuery, setSearchQuery] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lineItemCounter, setLineItemCounter] = useState(0);

  // Fetch products on mount and pre-populate selected products from order
  useEffect(() => {
    const fetchProducts = async () => {
      setIsLoading(true);
      try {
        const fetchedProducts = await productsApi.getAll();
        setProducts(fetchedProducts);

        // Pre-populate selected products from the order
        const initialSelectedProducts = new Map<string, ProductWithQuantity>();
        let counter = 0;
        openBill.products.forEach((orderProduct) => {
          const product = fetchedProducts.find(
            (p) => p.id === orderProduct.product.id,
          );
          if (product) {
            counter++;
            const lineItemId = `line-${counter}`;
            initialSelectedProducts.set(lineItemId, {
              lineItemId,
              openBillProductId: orderProduct.open_bill_product_id,
              product,
              quantity: orderProduct.quantity,
              notes: orderProduct.notes || "",
              status: orderProduct.status,
            });
          }
        });
        setSelectedProducts(initialSelectedProducts);
        setLineItemCounter(counter);
      } catch (err) {
        console.error("Error fetching products:", err);
        setError("Failed to load products");
      } finally {
        setIsLoading(false);
      }
    };
    fetchProducts();
  }, [openBill.products]);

  // Filter products based on search query
  const filteredProducts = useMemo(() => {
    if (!searchQuery.trim()) {
      return products;
    }
    const query = searchQuery.toLowerCase();
    return products.filter(
      (product) =>
        product.name.toLowerCase().includes(query) ||
        product.category.toLowerCase().includes(query),
    );
  }, [products, searchQuery]);

  const handleProductSelect = (product: Product) => {
    setLineItemCounter((prev) => {
      const lineItemId = `line-${prev + 1}`;
      const openBillProductId = crypto.randomUUID();
      setSelectedProducts((prevProducts) => {
        const newMap = new Map(prevProducts);
        newMap.set(lineItemId, {
          lineItemId,
          openBillProductId,
          product,
          quantity: 1,
          notes: "",
        });
        return newMap;
      });
      return prev + 1;
    });
  };

  const handleQuantityChange = (lineItemId: string, newQuantity: number) => {
    if (newQuantity <= 0) {
      setSelectedProducts((prev) => {
        const newMap = new Map(prev);
        newMap.delete(lineItemId);
        return newMap;
      });
    } else {
      setSelectedProducts((prev) => {
        const newMap = new Map(prev);
        const existing = newMap.get(lineItemId);
        if (existing) {
          newMap.set(lineItemId, {
            ...existing,
            quantity: newQuantity,
          });
        }
        return newMap;
      });
    }
  };

  const handleNotesChange = (lineItemId: string, notes: string) => {
    setSelectedProducts((prev) => {
      const newMap = new Map(prev);
      const existing = newMap.get(lineItemId);
      if (existing) {
        newMap.set(lineItemId, {
          ...existing,
          notes,
        });
      }
      return newMap;
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    setIsSubmitting(true);
    setError(null);

    try {
      const orderProducts: OrderProductItem[] = Array.from(
        selectedProducts.values(),
      ).map(({ openBillProductId, product, quantity, notes }) => ({
        open_bill_product_id: openBillProductId,
        product_id: product.id,
        quantity,
        notes: notes.trim() || null,
      }));

      await updateOrder(openBill.id, {
        temporal_identifier: temporalIdentifier.trim(),
        descriptor: descriptor.trim() || null,
        products: orderProducts,
      });

      onSuccess();
      onClose();
    } catch (err) {
      console.error("Error updating order:", err);
      setError(err instanceof Error ? err.message : "Failed to update order");
    } finally {
      setIsSubmitting(false);
    }
  };

  const selectedProductsArray = Array.from(selectedProducts.values());

  const groupedProducts = useMemo(() => {
    const productMap = new Map<
      string,
      {
        product: (typeof openBill.products)[0]["product"];
        totalQuantity: number;
      }
    >();
    openBill.products.forEach(({ product, quantity }) => {
      if (productMap.has(product.id)) {
        productMap.get(product.id)!.totalQuantity += quantity;
      } else {
        productMap.set(product.id, { product, totalQuantity: quantity });
      }
    });
    return Array.from(productMap.values());
  }, [openBill.products]);

  const printTotals = useMemo(() => {
    let subtotal = 0;
    let totalVAT = 0;
    let totalICO = 0;
    openBill.products.forEach(({ product, quantity }) => {
      subtotal += parseFloat(product.unit_price) * quantity;
      totalVAT += parseFloat(product.vat_amount || "0") * quantity;
      totalICO += parseFloat(product.ico_amount || "0") * quantity;
    });
    return { subtotal, totalVAT, totalICO, total: subtotal + totalVAT + totalICO };
  }, [openBill.products]);

  const handlePrint = () => {
    const printContent = printRef.current;
    if (!printContent) return;
    const printWindow = window.open("", "_blank");
    if (!printWindow) return;
    printWindow.document.write(
      generateInvoicePrintHTML({
        title: `Factura - ${openBill.temporal_identifier}`,
        content: printContent.innerHTML,
      }),
    );
    printWindow.document.close();
    printWindow.focus();
    setTimeout(() => {
      printWindow.print();
      printWindow.close();
    }, 250);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
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
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-lg)",
          maxWidth: "900px",
          width: "100%",
          maxHeight: "90vh",
          overflow: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            padding: "1.5rem",
            borderBottom: "1px solid var(--color-border)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            position: "sticky",
            top: 0,
            backgroundColor: "var(--color-surface)",
            zIndex: 1,
          }}
        >
          <div>
            <h2
              style={{
                margin: 0,
                fontSize: "1.5rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Editar Orden
            </h2>
            <div
              style={{
                marginTop: "0.5rem",
                fontSize: "0.9rem",
                color: "var(--color-text-secondary)",
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
              }}
            >
              {isEditingTitle ? (
                <input
                  autoFocus
                  type="text"
                  value={temporalIdentifier}
                  onChange={(e) => setTemporalIdentifier(e.target.value)}
                  onBlur={() => {
                    if (!temporalIdentifier.trim()) {
                      setTemporalIdentifier(openBill.temporal_identifier);
                    }
                    setIsEditingTitle(false);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      if (!temporalIdentifier.trim()) {
                        setTemporalIdentifier(openBill.temporal_identifier);
                      }
                      setIsEditingTitle(false);
                    }
                    if (e.key === "Escape") {
                      setTemporalIdentifier(openBill.temporal_identifier);
                      setIsEditingTitle(false);
                    }
                  }}
                  style={{
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-primary)",
                    padding: "0.25rem 0.5rem",
                    borderRadius: "var(--radius-sm)",
                    fontWeight: "bold",
                    fontSize: "0.9rem",
                    border: "1px solid var(--color-primary)",
                    outline: "none",
                    fontFamily: "inherit",
                    width: "auto",
                    minWidth: "80px",
                  }}
                />
              ) : (
                <span
                  style={{
                    backgroundColor: "var(--color-primary-light)",
                    color: "var(--color-primary)",
                    padding: "0.25rem 0.5rem",
                    borderRadius: "var(--radius-sm)",
                    fontWeight: "bold",
                  }}
                >
                  {temporalIdentifier}
                </span>
              )}
              <button
                type="button"
                onClick={() => setIsEditingTitle(true)}
                title="Editar identificador"
                style={{
                  background: "none",
                  border: "none",
                  cursor: "pointer",
                  padding: "0.2rem",
                  display: "flex",
                  alignItems: "center",
                  color: "var(--color-text-muted)",
                  borderRadius: "var(--radius-sm)",
                  transition: "color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.color = "var(--color-primary)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.color = "var(--color-text-muted)";
                }}
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
                  <path d="m15 5 4 4" />
                </svg>
              </button>
            </div>
          </div>
          <button
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              fontSize: "1.5rem",
              cursor: "pointer",
              color: "var(--color-text-secondary)",
              padding: "0.25rem",
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </div>

        {/* Order Total Summary */}
        <div
          style={{
            padding: "1rem 1.5rem",
            backgroundColor: "var(--color-primary-light)",
            borderBottom: "1px solid var(--color-border)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <span
            style={{
              fontSize: "1rem",
              fontWeight: "600",
              color: "var(--color-text-primary)",
            }}
          >
            Total de la Orden
          </span>
          <span
            style={{
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-primary)",
            }}
          >
            ${openBill.total_amount}
          </span>
        </div>

        {/* Quick Actions Bar */}
        <div
          style={{
            padding: "0.75rem 1.5rem",
            borderBottom: "1px solid var(--color-border)",
            display: "flex",
            gap: "0.5rem",
            backgroundColor: "var(--color-surface)",
          }}
        >
          <button
            type="button"
            onClick={handlePrint}
            style={{
              flex: 1,
              padding: "0.6rem 1rem",
              fontSize: "0.875rem",
              fontWeight: "bold",
              backgroundColor: "var(--color-primary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "0.4rem",
              transition: "background-color var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-primary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
            }}
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <polyline points="6 9 6 2 18 2 18 9" />
              <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2" />
              <rect x="6" y="14" width="12" height="8" />
            </svg>
            Imprimir
          </button>
          {onPayClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_UPDATE}>
              <button
                type="button"
                onClick={() => {
                  onClose();
                  onPayClick();
                }}
                style={{
                  flex: 1,
                  padding: "0.6rem 1rem",
                  fontSize: "0.875rem",
                  fontWeight: "bold",
                  backgroundColor: "var(--color-success)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: "0.4rem",
                  transition: "background-color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-success-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-success)";
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <line x1="12" y1="1" x2="12" y2="23" />
                  <path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
                </svg>
                Pagar Cuenta
              </button>
            </PermissionGate>
          )}
          {onRemoveClick && (
            <PermissionGate permission={PERMISSIONS.ORDERS_DELETE}>
              <button
                type="button"
                onClick={() => {
                  onClose();
                  onRemoveClick();
                }}
                style={{
                  padding: "0.6rem 1rem",
                  fontSize: "0.875rem",
                  fontWeight: "bold",
                  backgroundColor: "var(--color-danger)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-md)",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: "0.4rem",
                  transition: "background-color var(--transition-normal)",
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-danger-hover)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor =
                    "var(--color-danger)";
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="3 6 5 6 21 6" />
                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                </svg>
                Eliminar
              </button>
            </PermissionGate>
          )}
        </div>

        {/* Hidden Print Content */}
        <div style={{ display: "none" }}>
          <div ref={printRef}>
            <div className="header">
              <div className="bill-number">
                Factura - {openBill.temporal_identifier}
              </div>
              <div className="date">{formatDate(openBill.created_at)}</div>
              {openBill.created_by && (
                <div className="date">
                  Atendido por: {openBill.created_by.name}
                </div>
              )}
            </div>
            <div className="items">
              {groupedProducts.map(({ product, totalQuantity }) => {
                const itemTotal =
                  parseFloat(product.total_price_with_taxes) * totalQuantity;
                return (
                  <div key={product.id} className="item">
                    <div className="item-name">{product.name}</div>
                    <div className="item-details">
                      <span>
                        ${product.total_price_with_taxes} x {totalQuantity}
                      </span>
                      <span>${itemTotal.toFixed(2)}</span>
                    </div>
                  </div>
                );
              })}
            </div>
            <div className="totals">
              <div className="total-row">
                <span>Subtotal:</span>
                <span>${printTotals.subtotal.toFixed(2)}</span>
              </div>
              <div className="total-row">
                <span>IVA:</span>
                <span>${printTotals.totalVAT.toFixed(2)}</span>
              </div>
              <div className="total-row">
                <span>ICO:</span>
                <span>${printTotals.totalICO.toFixed(2)}</span>
              </div>
              <div className="total-row final">
                <span>TOTAL:</span>
                <span>${printTotals.total.toFixed(2)}</span>
              </div>
            </div>
            <div className="footer">
              <div>Gracias por su visita!</div>
              <div>Laguna Escondida</div>
            </div>
          </div>
        </div>

        <form onSubmit={handleSubmit}>
          {/* Form Body */}
          <div style={{ padding: "1.5rem" }}>
            {error && (
              <div
                style={{
                  padding: "1rem",
                  backgroundColor: "var(--color-danger-light)",
                  border: "1px solid var(--color-danger)",
                  borderRadius: "var(--radius-md)",
                  color: "var(--color-danger)",
                  marginBottom: "1rem",
                }}
              >
                {error}
              </div>
            )}

            {/* Order Info */}
            <div
              style={{
                marginBottom: "1.5rem",
                padding: "1rem",
                backgroundColor: "var(--color-bg)",
                borderRadius: "var(--radius-md)",
                border: "1px solid var(--color-border)",
              }}
            >
              <div style={{ marginBottom: "0.5rem" }}>
                <strong
                  style={{
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                  }}
                >
                  Creado por:{" "}
                </strong>
                <span style={{ color: "var(--color-text-primary)" }}>
                  {openBill.created_by?.name}
                </span>
              </div>
              <div>
                <strong
                  style={{
                    color: "var(--color-text-muted)",
                    fontSize: "0.875rem",
                  }}
                >
                  Creado el:{" "}
                </strong>
                <span style={{ color: "var(--color-text-secondary)" }}>
                  {new Date(openBill.created_at).toLocaleString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </span>
              </div>
            </div>

            {/* Descriptor */}
            <div style={{ marginBottom: "1.5rem" }}>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Descriptor (Opcional)
              </label>
              <textarea
                value={descriptor}
                onChange={(e) => setDescriptor(e.target.value)}
                placeholder="Descripción del cliente o mesa..."
                rows={3}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  outline: "none",
                  resize: "vertical",
                  fontFamily: "inherit",
                  backgroundColor: "var(--color-bg)",
                  color: "var(--color-text-primary)",
                }}
              />
            </div>

            {/* Selected Products */}
            {selectedProductsArray.length > 0 && (
              <div
                style={{
                  marginBottom: "1.5rem",
                  padding: "1rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-md)",
                  border: "1px solid var(--color-border)",
                }}
              >
                <h3
                  style={{
                    margin: "0 0 1rem 0",
                    fontSize: "1.1rem",
                    fontWeight: "bold",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Productos Agregados ({selectedProductsArray.length})
                </h3>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: "0.75rem",
                  }}
                >
                  {selectedProductsArray.map(
                    ({ lineItemId, product, quantity, notes, status }) => (
                      <div
                        key={lineItemId}
                        style={{
                          padding: "1rem",
                          backgroundColor: "var(--color-surface)",
                          borderRadius: "var(--radius-md)",
                          border: "1px solid var(--color-border)",
                        }}
                      >
                        <div
                          style={{
                            display: "flex",
                            justifyContent: "space-between",
                            alignItems: "center",
                            marginBottom: "0.75rem",
                          }}
                        >
                          <div>
                            <div
                              style={{
                                display: "flex",
                                alignItems: "center",
                                gap: "0.5rem",
                                marginBottom: "0.25rem",
                              }}
                            >
                              <strong
                                style={{ color: "var(--color-text-primary)" }}
                              >
                                {product.name}
                              </strong>
                              {status && (
                                <span
                                  style={{
                                    fontSize: "0.7rem",
                                    fontWeight: "600",
                                    padding: "0.15rem 0.5rem",
                                    borderRadius: "var(--radius-sm)",
                                    backgroundColor:
                                      STATUS_CONFIG[status].bgColor,
                                    color: STATUS_CONFIG[status].textColor,
                                    textTransform: "uppercase",
                                    letterSpacing: "0.3px",
                                  }}
                                >
                                  {STATUS_CONFIG[status].label}
                                </span>
                              )}
                            </div>
                            <div
                              style={{
                                fontSize: "0.875rem",
                                color: "var(--color-text-secondary)",
                              }}
                            >
                              ${product.total_price_with_taxes}
                            </div>
                          </div>
                          <div
                            style={{
                              display: "flex",
                              alignItems: "center",
                              gap: "0.5rem",
                            }}
                          >
                            <button
                              type="button"
                              onClick={() =>
                                handleQuantityChange(lineItemId, quantity - 1)
                              }
                              style={{
                                width: "32px",
                                height: "32px",
                                border: "1px solid var(--color-primary)",
                                borderRadius: "var(--radius-sm)",
                                backgroundColor: "transparent",
                                color: "var(--color-primary)",
                                cursor: "pointer",
                                fontSize: "1.25rem",
                                lineHeight: 1,
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                              }}
                            >
                              −
                            </button>
                            <input
                              type="number"
                              value={quantity}
                              onChange={(e) => {
                                const val = e.target.value;
                                if (val === "") return;
                                const parsed = parseInt(val, 10);
                                if (!isNaN(parsed) && parsed >= 0) {
                                  handleQuantityChange(lineItemId, parsed);
                                }
                              }}
                              onBlur={(e) => {
                                const val = e.target.value;
                                if (val === "" || parseInt(val, 10) <= 0) {
                                  handleQuantityChange(lineItemId, 0);
                                }
                              }}
                              min="1"
                              style={{
                                width: "70px",
                                textAlign: "center",
                                fontWeight: "bold",
                                fontSize: "1.1rem",
                                color: "var(--color-text-primary)",
                                border: "1px solid var(--color-border)",
                                borderRadius: "var(--radius-sm)",
                                padding: "0.25rem",
                                backgroundColor: "var(--color-bg)",
                                outline: "none",
                              }}
                            />
                            <button
                              type="button"
                              onClick={() =>
                                handleQuantityChange(lineItemId, quantity + 1)
                              }
                              style={{
                                width: "32px",
                                height: "32px",
                                border: "1px solid var(--color-primary)",
                                borderRadius: "var(--radius-sm)",
                                backgroundColor: "transparent",
                                color: "var(--color-primary)",
                                cursor: "pointer",
                                fontSize: "1.25rem",
                                lineHeight: 1,
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                              }}
                            >
                              +
                            </button>
                          </div>
                        </div>
                        <input
                          type="text"
                          value={notes}
                          onChange={(e) =>
                            handleNotesChange(lineItemId, e.target.value)
                          }
                          placeholder="Agregar notas (opcional)..."
                          style={{
                            width: "100%",
                            padding: "0.5rem",
                            fontSize: "0.875rem",
                            border: "1px solid var(--color-border)",
                            borderRadius: "var(--radius-sm)",
                            outline: "none",
                            backgroundColor: "var(--color-bg)",
                            color: "var(--color-text-primary)",
                          }}
                        />
                      </div>
                    ),
                  )}
                </div>
              </div>
            )}

            {/* Product Search and Selection */}
            <div>
              <h3
                style={{
                  margin: "0 0 0.75rem 0",
                  fontSize: "1.1rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Agregar Más Productos
              </h3>
              <input
                type="text"
                placeholder="Buscar productos..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  outline: "none",
                  marginBottom: "1rem",
                  backgroundColor: "var(--color-bg)",
                  color: "var(--color-text-primary)",
                }}
              />
              <div
                style={{
                  maxHeight: "300px",
                  overflowY: "auto",
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))",
                  gap: "0.75rem",
                }}
              >
                {isLoading ? (
                  <div
                    style={{
                      padding: "2rem",
                      textAlign: "center",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    Cargando productos...
                  </div>
                ) : filteredProducts.length === 0 ? (
                  <div
                    style={{
                      padding: "2rem",
                      textAlign: "center",
                      color: "var(--color-text-secondary)",
                    }}
                  >
                    No se encontraron productos
                  </div>
                ) : (
                  filteredProducts.map((product) => {
                    return (
                      <div
                        key={product.id}
                        onClick={() => handleProductSelect(product)}
                        style={{
                          padding: "1rem",
                          border: "1px solid var(--color-border)",
                          borderRadius: "var(--radius-md)",
                          cursor: "pointer",
                          backgroundColor: "var(--color-surface)",
                          transition: "all var(--transition-normal)",
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.borderColor =
                            "var(--color-primary)";
                          e.currentTarget.style.backgroundColor =
                            "var(--color-surface-hover)";
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.borderColor =
                            "var(--color-border)";
                          e.currentTarget.style.backgroundColor =
                            "var(--color-surface)";
                        }}
                      >
                        <div
                          style={{
                            fontWeight: "bold",
                            color: "var(--color-text-primary)",
                            marginBottom: "0.25rem",
                            fontSize: "0.95rem",
                          }}
                        >
                          {product.name}
                        </div>
                        <div
                          style={{
                            fontSize: "0.8rem",
                            color: "var(--color-text-muted)",
                            marginBottom: "0.5rem",
                          }}
                        >
                          {product.category}
                        </div>
                        <div
                          style={{
                            fontSize: "0.9rem",
                            fontWeight: "bold",
                            color: "var(--color-success)",
                          }}
                        >
                          ${product.total_price_with_taxes}
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            </div>
          </div>

          {/* Footer */}
          <div
            style={{
              padding: "1.5rem",
              borderTop: "1px solid var(--color-border)",
              display: "flex",
              gap: "1rem",
              justifyContent: "flex-end",
              position: "sticky",
              bottom: 0,
              backgroundColor: "var(--color-surface)",
            }}
          >
            <button
              type="button"
              onClick={onClose}
              style={{
                padding: "0.75rem 1.5rem",
                fontSize: "1rem",
                fontWeight: "bold",
                backgroundColor: "var(--color-surface-hover)",
                color: "var(--color-text-primary)",
                border: "1px solid var(--color-border)",
                borderRadius: "var(--radius-md)",
                cursor: "pointer",
              }}
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              style={{
                padding: "0.75rem 1.5rem",
                fontSize: "1rem",
                fontWeight: "bold",
                backgroundColor: isSubmitting
                  ? "var(--color-text-muted)"
                  : "var(--color-primary)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-md)",
                cursor: isSubmitting ? "not-allowed" : "pointer",
              }}
            >
              {isSubmitting ? "Actualizando..." : "Actualizar Orden"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
