"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import { productsApi } from "@/lib/api/products";
import { updateOrder } from "@/lib/api/orders";
import { printTicket } from "@/lib/api/device";
import { generateInvoicePrintHTML } from "@/lib/templates/invoicePrint";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { canOverrideItemLock, isItemPastGrace } from "@/lib/orders/itemLock";
import { Button, Input, Textarea } from "@/components/ui";
import {
  ProductCard,
  OrderLineItem,
  CategoryPills,
  formatCOP,
  sendIcon,
} from "./orderTakingParts";
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
  /**
   * The item's own creation time (ISO 8601), for persisted items only. Left
   * undefined for items the waitress adds in the current session — those are
   * always freely editable. Drives the 5-minute edit lock.
   */
  createdAt?: string;
}

/**
 * A stable, order-independent fingerprint of the editable state (identifier,
 * descriptor, and every line's product/quantity/notes). We compare the live
 * fingerprint against the one captured at load/last-save to know whether the
 * waitress has actually changed anything — which drives the update button's
 * gray→green colour.
 */
function orderSignature(
  identifier: string,
  descriptor: string,
  items: ProductWithQuantity[],
): string {
  const lines = items
    .map((i) => `${i.product.id}|${i.quantity}|${i.notes.trim()}`)
    .sort()
    .join("§");
  return `${identifier.trim()}¦${(descriptor || "").trim()}¦${lines}`;
}

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
  const [isPrinting, setIsPrinting] = useState(false);
  const [products, setProducts] = useState<Product[]>([]);
  const [selectedProducts, setSelectedProducts] = useState<
    Map<string, ProductWithQuantity>
  >(new Map());
  const [searchQuery, setSearchQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState<string>("all");
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lineItemCounter, setLineItemCounter] = useState(0);
  // Fingerprint of the order as last saved (or as loaded). `null` until the
  // products finish loading so the button stays gray during the initial fetch.
  const [baselineSignature, setBaselineSignature] = useState<string | null>(
    null,
  );

  // Who's editing — admins/managers bypass the per-item edit lock entirely.
  const { user } = usePermissions();
  const canOverride = canOverrideItemLock(user);

  // A ticking "now" so a line locks the instant it crosses the 5-minute window,
  // even while the waitress just sits in an open modal. 30s granularity is
  // plenty for a 5-minute rule.
  const [now, setNow] = useState<number>(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  // A persisted line past its grace window is locked for non-managers. Session
  // items (no createdAt) are never locked. Uses a fresh clock at mutation time.
  const isLineLocked = (
    item: ProductWithQuantity,
    atMs: number = Date.now(),
  ): boolean => !canOverride && isItemPastGrace(item.createdAt, atMs);

  // Mobile layout: below 768px we render the Menú / Pedido tabs (Option B); the
  // desktop two-pane POS renders unchanged above it. `mobileView` toggles the tabs.
  const [isMobile, setIsMobile] = useState(false);
  const [mobileView, setMobileView] = useState<"menu" | "order">("menu");
  useEffect(() => {
    const mq = window.matchMedia("(max-width: 767px)");
    const update = () => setIsMobile(mq.matches);
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, []);

  // Esc to close + body scroll lock (mirrors the Modal primitive). Esc is a no-op
  // while inline-editing the identifier so it only cancels that edit.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !isEditingTitle) onClose();
    };
    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [onClose, isEditingTitle]);

  // Fetch products on mount and pre-populate selected products from order
  useEffect(() => {
    const fetchProducts = async () => {
      setIsLoading(true);
      try {
        // Only show products that can be sold to customers
        const fetchedProducts = await productsApi.getAll("SELLABLE");
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
              createdAt: orderProduct.created_at,
            });
          }
        });
        if (
          openBill.products.length > 0 &&
          !openBill.products.some((p) => p.created_at)
        ) {
          // The 5-minute item lock silently no-ops without per-item timestamps.
          console.warn(
            "[EditOrderForm] Open bill products have no `created_at`; the item edit lock cannot apply. Ensure GET /orders/:id returns created_at per product.",
          );
        }
        setSelectedProducts(initialSelectedProducts);
        setLineItemCounter(counter);
        setBaselineSignature(
          orderSignature(
            openBill.temporal_identifier,
            openBill.descriptor || "",
            Array.from(initialSelectedProducts.values()),
          ),
        );
      } catch (err) {
        console.error("Error fetching products:", err);
        setError("Failed to load products");
      } finally {
        setIsLoading(false);
      }
    };
    fetchProducts();
  }, [openBill.products]);

  // Unique categories for the filter pills
  const categories = useMemo(() => {
    return Array.from(
      new Set(products.map((p) => p.category).filter(Boolean)),
    ).sort();
  }, [products]);

  // Filter products by active category pill and search query
  const filteredProducts = useMemo(() => {
    let list = products;
    if (activeCategory !== "all") {
      list = list.filter((product) => product.category === activeCategory);
    }
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      list = list.filter(
        (product) =>
          product.name.toLowerCase().includes(query) ||
          product.category.toLowerCase().includes(query),
      );
    }
    return list;
  }, [products, searchQuery, activeCategory]);

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
    const target = selectedProducts.get(lineItemId);
    if (target && isLineLocked(target)) return;
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
    const target = selectedProducts.get(lineItemId);
    if (target && isLineLocked(target)) return;
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

      // Keep the modal open so the waitress doesn't lose their place hunting for
      // the order they just touched. Re-baseline to the just-saved state so the
      // button drops back to gray until they change something else.
      setBaselineSignature(
        orderSignature(
          temporalIdentifier,
          descriptor,
          Array.from(selectedProducts.values()),
        ),
      );
      onSuccess();
    } catch (err) {
      console.error("Error updating order:", err);
      setError(err instanceof Error ? err.message : "Failed to update order");
    } finally {
      setIsSubmitting(false);
    }
  };

  const selectedProductsArray = Array.from(selectedProducts.values());
  const orderCount = selectedProductsArray.length;

  // Has the waitress changed anything since load / last save? Drives the update
  // button colour: gray when nothing's changed, green once there are edits.
  const currentSignature = useMemo(
    () => orderSignature(temporalIdentifier, descriptor, selectedProductsArray),
    [temporalIdentifier, descriptor, selectedProductsArray],
  );
  const isDirty =
    baselineSignature !== null && currentSignature !== baselineSignature;

  // Gray (pristine) vs green (has edits). Inline styles override the Button's
  // primary variant background.
  const updateButtonStyle: React.CSSProperties = isDirty
    ? {
        backgroundColor: "var(--color-success)",
        borderColor: "var(--color-success)",
        color: "#fff",
      }
    : {
        backgroundColor: "var(--color-neutral-bg)",
        borderColor: "var(--color-border)",
        color: "var(--color-text-muted)",
      };
  const qtyInOrderFor = (productId: string) =>
    selectedProductsArray
      .filter((p) => p.product.id === productId)
      .reduce((sum, p) => sum + p.quantity, 0);

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

  // Browser fallback: opens a print window with the rendered HTML. Used when the
  // edge printer endpoint is unavailable (e.g. cloud mode, or printer offline).
  const browserPrint = () => {
    const printContentEl = printRef.current;
    if (!printContentEl) return;
    const printWindow = window.open("", "_blank");
    if (!printWindow) return;
    printWindow.document.write(
      generateInvoicePrintHTML({
        title: `Factura - ${openBill.temporal_identifier}`,
        content: printContentEl.innerHTML,
      }),
    );
    printWindow.document.close();
    printWindow.focus();
    setTimeout(() => {
      printWindow.print();
      printWindow.close();
    }, 250);
  };

  // Print on the edge node's physical receipt printer via POST /api/device/print.
  // Falls back to browser printing if the endpoint is unavailable or the printer
  // cannot be reached.
  const handlePrint = async () => {
    if (isPrinting) return;
    setIsPrinting(true);
    try {
      await printTicket({ open_bill_id: openBill.id });
    } catch (err) {
      console.error("Edge print failed, falling back to browser print:", err);
      browserPrint();
    } finally {
      setIsPrinting(false);
    }
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

  const createdChip = new Date(openBill.created_at).toLocaleString("es-CO", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });

  // Inline-editable identifier (shared by both layouts)
  const identifierBlock = (
    <div
      style={{
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
            fontWeight: 800,
            fontSize: "1.15rem",
            border: "1px solid var(--color-primary)",
            outline: "none",
            fontFamily: "inherit",
            width: "auto",
            minWidth: "120px",
          }}
        />
      ) : (
        <h2
          style={{
            margin: 0,
            fontSize: "1.25rem",
            fontWeight: 800,
            letterSpacing: "-0.01em",
            color: "var(--color-text-primary)",
            whiteSpace: "nowrap",
            overflow: "hidden",
            textOverflow: "ellipsis",
          }}
        >
          {temporalIdentifier}
        </h2>
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
        }}
      >
        <svg
          width="15"
          height="15"
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
  );

  // Hidden print content (shared by both layouts) — referenced by handlePrint via printRef
  const printContent = (
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
  );

  // ───────────────────────────── Mobile (Option B) ─────────────────────────────
  if (isMobile) {
    return (
      <div
        style={{
          position: "fixed",
          inset: 0,
          backgroundColor: "var(--color-overlay)",
          display: "flex",
          zIndex: 1000,
        }}
        onClick={onClose}
      >
        <div
          role="dialog"
          aria-modal="true"
          onClick={(e) => e.stopPropagation()}
          style={{
            backgroundColor: "var(--color-surface)",
            width: "100%",
            height: "100%",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          {printContent}

          {/* Header */}
          <header
            style={{
              flex: "none",
              display: "flex",
              alignItems: "center",
              gap: "0.75rem",
              padding: "0.75rem 0.875rem",
              borderBottom: "1px solid var(--color-border)",
            }}
          >
            <button
              type="button"
              onClick={onClose}
              title="Volver"
              style={{
                width: "38px",
                height: "38px",
                flex: "none",
                borderRadius: "var(--radius-md)",
                border: "1px solid var(--color-border)",
                backgroundColor: "var(--color-surface)",
                color: "var(--color-text-secondary)",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <svg
                width="19"
                height="19"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2.4"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <polyline points="15 18 9 12 15 6" />
              </svg>
            </button>
            <div style={{ minWidth: 0, flex: 1 }}>
              {identifierBlock}
              {openBill.created_by?.name && (
                <div
                  style={{
                    fontSize: "0.78rem",
                    color: "var(--color-text-secondary)",
                    marginTop: "0.1rem",
                  }}
                >
                  Mesero: {openBill.created_by.name} · {createdChip}
                </div>
              )}
            </div>
          </header>

          {/* Segmented control */}
          <div
            style={{
              flex: "none",
              display: "flex",
              gap: "4px",
              margin: "0.6rem 0.875rem",
              padding: "4px",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-md)",
            }}
          >
            {(["menu", "order"] as const).map((v) => {
              const on = mobileView === v;
              return (
                <button
                  key={v}
                  type="button"
                  onClick={() => setMobileView(v)}
                  style={{
                    flex: 1,
                    border: "none",
                    cursor: "pointer",
                    fontFamily: "inherit",
                    fontSize: "0.9rem",
                    fontWeight: 700,
                    padding: "0.55rem",
                    borderRadius: "var(--radius-sm)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    gap: "0.4rem",
                    backgroundColor: on ? "var(--color-surface)" : "transparent",
                    color: on
                      ? "var(--color-text-primary)"
                      : "var(--color-text-secondary)",
                    boxShadow: on ? "var(--shadow-sm)" : "none",
                  }}
                >
                  {v === "menu" ? "Menú" : "Pedido"}
                  {v === "order" && orderCount > 0 && (
                    <span
                      style={{
                        backgroundColor: "var(--color-primary)",
                        color: "white",
                        fontSize: "0.7rem",
                        fontWeight: 800,
                        minWidth: "18px",
                        height: "18px",
                        padding: "0 5px",
                        borderRadius: "999px",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      {orderCount}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {error && (
            <div
              style={{
                flex: "none",
                margin: "0 0.875rem 0.5rem",
                padding: "0.625rem 0.875rem",
                backgroundColor: "var(--color-danger-light)",
                border: "1px solid var(--color-danger)",
                borderRadius: "var(--radius-md)",
                color: "var(--color-danger)",
                fontWeight: 500,
                fontSize: "0.85rem",
              }}
            >
              {error}
            </div>
          )}

          <form
            onSubmit={handleSubmit}
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              minHeight: 0,
            }}
          >
            {mobileView === "menu" ? (
              <>
                {/* Search */}
                <div style={{ flex: "none", padding: "0 0.875rem 0.6rem" }}>
                  <Input
                    type="search"
                    placeholder="Buscar producto..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                </div>
                {/* Category pills */}
                {categories.length > 0 && (
                  <div style={{ flex: "none", padding: "0 0.875rem 0.6rem" }}>
                    <CategoryPills
                      categories={categories}
                      active={activeCategory}
                      onChange={setActiveCategory}
                      compact
                    />
                  </div>
                )}
                {/* Product grid (2-col) */}
                <div
                  style={{
                    flex: 1,
                    overflowY: "auto",
                    minHeight: 0,
                    padding: "0 0.875rem 0.875rem",
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr",
                    gridAutoRows: "min-content",
                    gap: "0.6rem",
                  }}
                >
                  {isLoading ? (
                    <div
                      style={{
                        gridColumn: "1 / -1",
                        padding: "2.5rem",
                        textAlign: "center",
                        color: "var(--color-text-secondary)",
                      }}
                    >
                      Cargando productos...
                    </div>
                  ) : filteredProducts.length === 0 ? (
                    <div
                      style={{
                        gridColumn: "1 / -1",
                        padding: "2.5rem",
                        textAlign: "center",
                        color: "var(--color-text-secondary)",
                      }}
                    >
                      No se encontraron productos
                    </div>
                  ) : (
                    filteredProducts.map((product) => (
                      <ProductCard
                        key={product.id}
                        product={product}
                        qtyInOrder={qtyInOrderFor(product.id)}
                        onAdd={handleProductSelect}
                      />
                    ))
                  )}
                </div>
              </>
            ) : (
              <>
                {/* Descriptor */}
                <div style={{ flex: "none", padding: "0 0.875rem 0.6rem" }}>
                  <Textarea
                    value={descriptor}
                    onChange={(e) => setDescriptor(e.target.value)}
                    placeholder="Descriptor (opcional): cliente, mesa..."
                    rows={2}
                  />
                </div>
                {/* Items */}
                <div
                  style={{
                    flex: 1,
                    overflowY: "auto",
                    minHeight: 0,
                    padding: "0 0.875rem",
                  }}
                >
                  {orderCount === 0 ? (
                    <div
                      style={{
                        padding: "2.5rem 1rem",
                        textAlign: "center",
                        color: "var(--color-text-muted)",
                        fontSize: "0.9rem",
                      }}
                    >
                      Toca <strong>Menú</strong> para agregar productos.
                    </div>
                  ) : (
                    selectedProductsArray.map((item) => (
                      <OrderLineItem
                        key={item.lineItemId}
                        item={item}
                        locked={isLineLocked(item, now)}
                        onQuantityChange={handleQuantityChange}
                        onNotesChange={handleNotesChange}
                      />
                    ))
                  )}
                </div>
                {/* Quick actions */}
                <div
                  style={{
                    flex: "none",
                    display: "flex",
                    gap: "0.5rem",
                    flexWrap: "wrap",
                    padding: "0.6rem 0.875rem",
                    borderTop: "1px solid var(--color-border)",
                  }}
                >
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={handlePrint}
                    disabled={isPrinting}
                  >
                    {isPrinting ? "Imprimiendo..." : "Imprimir"}
                  </Button>
                  {onPayClick && (
                    <PermissionGate permission={PERMISSIONS.ORDERS_PAY}>
                      <Button
                        type="button"
                        size="sm"
                        onClick={() => {
                          onClose();
                          onPayClick();
                        }}
                      >
                        Pagar Cuenta
                      </Button>
                    </PermissionGate>
                  )}
                  {onRemoveClick && (
                    <PermissionGate permission={PERMISSIONS.ORDERS_DELETE}>
                      <Button
                        type="button"
                        variant="danger"
                        size="sm"
                        onClick={() => {
                          onClose();
                          onRemoveClick();
                        }}
                      >
                        Eliminar
                      </Button>
                    </PermissionGate>
                  )}
                </div>
              </>
            )}

            {/* Footer: total + submit */}
            <div
              style={{
                flex: "none",
                borderTop: "1px solid var(--color-border)",
                padding: "0.75rem 0.875rem 0.9rem",
                backgroundColor: "var(--color-bg)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "baseline",
                  marginBottom: "0.6rem",
                }}
              >
                <span
                  style={{
                    fontSize: "0.95rem",
                    fontWeight: 700,
                    color: "var(--color-text-primary)",
                  }}
                >
                  Total
                </span>
                <span
                  style={{
                    fontSize: "1.35rem",
                    fontWeight: 800,
                    color: "var(--color-primary)",
                    fontVariantNumeric: "tabular-nums",
                  }}
                >
                  {formatCOP(openBill.total_amount)}
                </span>
              </div>
              <Button
                type="submit"
                fullWidth
                size="lg"
                disabled={isSubmitting}
                leftIcon={sendIcon}
                style={updateButtonStyle}
              >
                {isSubmitting ? "Actualizando..." : "Actualizar Orden"}
              </Button>
            </div>
          </form>
        </div>
      </div>
    );
  }

  // ───────────────────────────── Desktop (two-pane POS) ─────────────────────────
  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
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
        role="dialog"
        aria-modal="true"
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-lg)",
          width: "min(1280px, 96vw)",
          height: "92vh",
          maxHeight: "92vh",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <header
          style={{
            flex: "none",
            display: "flex",
            alignItems: "center",
            gap: "0.875rem",
            padding: "0.875rem 1.25rem",
            borderBottom: "1px solid var(--color-border)",
            backgroundColor: "var(--color-surface)",
          }}
        >
          <button
            type="button"
            onClick={onClose}
            title="Volver"
            style={{
              width: "42px",
              height: "42px",
              flex: "none",
              borderRadius: "var(--radius-md)",
              border: "1px solid var(--color-border)",
              backgroundColor: "var(--color-surface)",
              color: "var(--color-text-secondary)",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.4"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <polyline points="15 18 9 12 15 6" />
            </svg>
          </button>

          {/* Editable identifier + meta */}
          <div style={{ minWidth: 0 }}>
            {identifierBlock}
            {openBill.created_by?.name && (
              <div
                style={{
                  fontSize: "0.8rem",
                  color: "var(--color-text-secondary)",
                  marginTop: "0.15rem",
                }}
              >
                Mesero: {openBill.created_by.name} · {createdChip}
              </div>
            )}
          </div>

          {/* Actions */}
          <div
            style={{
              marginLeft: "auto",
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              flexWrap: "wrap",
            }}
          >
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handlePrint}
              disabled={isPrinting}
              leftIcon={
                <svg
                  width="15"
                  height="15"
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
              }
            >
              {isPrinting ? "Imprimiendo..." : "Imprimir"}
            </Button>
            {onPayClick && (
              <PermissionGate permission={PERMISSIONS.ORDERS_PAY}>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => {
                    onClose();
                    onPayClick();
                  }}
                  leftIcon={
                    <svg
                      width="15"
                      height="15"
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
                  }
                >
                  Pagar Cuenta
                </Button>
              </PermissionGate>
            )}
            {onRemoveClick && (
              <PermissionGate permission={PERMISSIONS.ORDERS_DELETE}>
                <Button
                  type="button"
                  variant="danger"
                  size="sm"
                  onClick={() => {
                    onClose();
                    onRemoveClick();
                  }}
                  leftIcon={
                    <svg
                      width="15"
                      height="15"
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
                  }
                >
                  Eliminar
                </Button>
              </PermissionGate>
            )}
          </div>
        </header>

        {printContent}

        {error && (
          <div
            style={{
              flex: "none",
              padding: "0.75rem 1.25rem",
              backgroundColor: "var(--color-danger-light)",
              borderBottom: "1px solid var(--color-danger)",
              color: "var(--color-danger)",
              fontWeight: 500,
            }}
          >
            {error}
          </div>
        )}

        {/* Two-pane body */}
        <form
          onSubmit={handleSubmit}
          style={{ flex: 1, display: "flex", minHeight: 0 }}
        >
          {/* Left: menu */}
          <section
            style={{
              flex: 1,
              minWidth: 0,
              display: "flex",
              flexDirection: "column",
              padding: "1.125rem 1.25rem",
            }}
          >
            {/* Search + category pills */}
            <div style={{ flex: "none", marginBottom: "1rem" }}>
              <Input
                type="search"
                placeholder="Buscar productos..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              <div style={{ marginTop: "0.875rem" }}>
                <CategoryPills
                  categories={categories}
                  active={activeCategory}
                  onChange={setActiveCategory}
                  compact={false}
                />
              </div>
            </div>

            {/* Product grid */}
            <div
              style={{
                flex: 1,
                overflowY: "auto",
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))",
                gridAutoRows: "min-content",
                gap: "1rem",
                paddingBottom: "0.25rem",
              }}
            >
              {isLoading ? (
                <div
                  style={{
                    gridColumn: "1 / -1",
                    padding: "3rem",
                    textAlign: "center",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  Cargando productos...
                </div>
              ) : filteredProducts.length === 0 ? (
                <div
                  style={{
                    gridColumn: "1 / -1",
                    padding: "3rem",
                    textAlign: "center",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  No se encontraron productos
                </div>
              ) : (
                filteredProducts.map((product) => (
                  <ProductCard
                    key={product.id}
                    product={product}
                    qtyInOrder={qtyInOrderFor(product.id)}
                    onAdd={handleProductSelect}
                  />
                ))
              )}
            </div>
          </section>

          {/* Right: order sidebar */}
          <aside
            style={{
              width: "360px",
              flex: "none",
              display: "flex",
              flexDirection: "column",
              borderLeft: "1px solid var(--color-border)",
              backgroundColor: "var(--color-surface)",
              minHeight: 0,
            }}
          >
            {/* Sidebar header */}
            <div
              style={{
                flex: "none",
                padding: "1.125rem 1.25rem 0.875rem",
                borderBottom: "1px solid var(--color-border)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  marginBottom: "0.75rem",
                }}
              >
                <h3
                  style={{
                    margin: 0,
                    fontSize: "1.05rem",
                    fontWeight: 700,
                    color: "var(--color-text-primary)",
                  }}
                >
                  Pedido actual
                </h3>
                <span
                  style={{
                    fontSize: "0.8rem",
                    color: "var(--color-text-secondary)",
                  }}
                >
                  {orderCount} {orderCount === 1 ? "producto" : "productos"}
                </span>
              </div>
              <Textarea
                value={descriptor}
                onChange={(e) => setDescriptor(e.target.value)}
                placeholder="Descriptor (opcional): cliente, mesa..."
                rows={2}
              />
            </div>

            {/* Items */}
            <div
              style={{
                flex: 1,
                overflowY: "auto",
                padding: "0.5rem 1rem",
                minHeight: 0,
              }}
            >
              {orderCount === 0 ? (
                <div
                  style={{
                    padding: "2.5rem 1rem",
                    textAlign: "center",
                    color: "var(--color-text-muted)",
                    fontSize: "0.9rem",
                  }}
                >
                  Toca un producto para agregarlo al pedido.
                </div>
              ) : (
                selectedProductsArray.map((item) => (
                  <OrderLineItem
                    key={item.lineItemId}
                    item={item}
                    locked={isLineLocked(item, now)}
                    onQuantityChange={handleQuantityChange}
                    onNotesChange={handleNotesChange}
                  />
                ))
              )}
            </div>

            {/* Footer: total + submit */}
            <div
              style={{
                flex: "none",
                borderTop: "1px solid var(--color-border)",
                padding: "1rem 1.25rem 1.125rem",
                backgroundColor: "var(--color-bg)",
              }}
            >
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "baseline",
                  marginBottom: "0.875rem",
                }}
              >
                <span
                  style={{
                    fontSize: "1rem",
                    fontWeight: 700,
                    color: "var(--color-text-primary)",
                  }}
                >
                  Total
                </span>
                <span
                  style={{
                    fontSize: "1.4rem",
                    fontWeight: 800,
                    color: "var(--color-primary)",
                    fontVariantNumeric: "tabular-nums",
                  }}
                >
                  {formatCOP(openBill.total_amount)}
                </span>
              </div>
              <Button
                type="submit"
                fullWidth
                size="lg"
                disabled={isSubmitting}
                leftIcon={sendIcon}
                style={updateButtonStyle}
              >
                {isSubmitting ? "Actualizando..." : "Actualizar Orden"}
              </Button>
            </div>
          </aside>
        </form>
      </div>
    </div>
  );
}
