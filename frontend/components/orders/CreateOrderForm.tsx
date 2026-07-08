"use client";

import { useState, useEffect, useMemo } from "react";
import { productsApi } from "@/lib/api/products";
import { createOrder } from "@/lib/api/orders";
import type { Product } from "@/types/product";
import type { OrderProductItem } from "@/types/order";
import { Button, Input, Textarea } from "@/components/ui";
import {
  ProductCard,
  OrderLineItem,
  CategoryPills,
  formatCOP,
  sendIcon,
  isWeighedProduct,
} from "./orderTakingParts";

// Prefix pre-filled into the temporal identifier input so the waitress only
// types the table number after it (e.g. "MESA-12"). This is a frontend-only
// convention: the backend accepts any temporal identifier.
const TEMPORAL_IDENTIFIER_PREFIX = "MESA-";

interface CreateOrderFormProps {
  onClose: () => void;
  onSuccess: () => void;
}

interface ProductWithQuantity {
  lineItemId: string;
  openBillProductId: string;
  product: Product;
  quantity: number;
  notes: string;
}

export default function CreateOrderForm({
  onClose,
  onSuccess,
}: CreateOrderFormProps) {
  const [openBillId] = useState(() => crypto.randomUUID());
  const [temporalIdentifier, setTemporalIdentifier] = useState(
    TEMPORAL_IDENTIFIER_PREFIX
  );
  const [descriptor, setDescriptor] = useState("");
  const [products, setProducts] = useState<Product[]>([]);
  const [selectedProducts, setSelectedProducts] = useState<
    Map<string, ProductWithQuantity>
  >(new Map());
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lineItemCounter, setLineItemCounter] = useState(0);
  // Bumping `focusNonce` re-fires a weighed line's focus effect; `focusLineItemId`
  // says which line to focus. Set when a weighed product is (re)tapped in the menu.
  const [focusNonce, setFocusNonce] = useState(0);
  const [focusLineItemId, setFocusLineItemId] = useState<string | null>(null);

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

  // Esc to close + body scroll lock (mirrors the Modal primitive)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [onClose]);

  // Fetch products on mount
  useEffect(() => {
    const fetchProducts = async () => {
      setIsLoading(true);
      try {
        // Only show products that can be sold to customers
        const fetchedProducts = await productsApi.getAll("SELLABLE");
        setProducts(fetchedProducts);
      } catch (err) {
        console.error("Error fetching products:", err);
        setError("Failed to load products");
      } finally {
        setIsLoading(false);
      }
    };
    fetchProducts();
  }, []);

  // Extract unique categories from products
  const categories = useMemo(() => {
    const uniqueCategories = Array.from(
      new Set(products.map((p) => p.category))
    ).sort();
    return uniqueCategories;
  }, [products]);

  // Filter products based on search query and category
  const filteredProducts = useMemo(() => {
    let filtered = products;

    // Filter by category
    if (selectedCategory !== "all") {
      filtered = filtered.filter(
        (product) => product.category === selectedCategory
      );
    }

    // Filter by search query
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (product) =>
          product.name.toLowerCase().includes(query) ||
          product.category.toLowerCase().includes(query)
      );
    }

    return filtered;
  }, [products, searchQuery, selectedCategory]);

  const handleProductSelect = (product: Product) => {
    const weighed = isWeighedProduct(product);
    // Weighed products live on a single line — re-tapping focuses the existing
    // line instead of adding a duplicate.
    if (weighed) {
      const existing = Array.from(selectedProducts.values()).find(
        (p) => p.product.id === product.id,
      );
      if (existing) {
        setFocusLineItemId(existing.lineItemId);
        setFocusNonce((n) => n + 1);
        return;
      }
    }
    setLineItemCounter((prev) => {
      const lineItemId = `line-${prev + 1}`;
      const openBillProductId = crypto.randomUUID();
      setSelectedProducts((prevProducts) => {
        const newMap = new Map(prevProducts);
        newMap.set(lineItemId, {
          lineItemId,
          openBillProductId,
          product,
          // Weighed lines start with no weight (pending); the waitress types the
          // scale reading. Unit products start at 1.
          quantity: weighed ? 0 : 1,
          notes: "",
        });
        return newMap;
      });
      if (weighed) {
        setFocusLineItemId(lineItemId);
        setFocusNonce((n) => n + 1);
      }
      return prev + 1;
    });
  };

  const handleQuantityChange = (lineItemId: string, newQuantity: number) => {
    const target = selectedProducts.get(lineItemId);
    const weighed = target ? isWeighedProduct(target.product) : false;
    // A unit line hitting 0 is removed; a weighed line stays as "pending" (no
    // weight yet) and is only removed via handleRemove.
    if (newQuantity <= 0 && !weighed) {
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
            quantity: Math.max(0, newQuantity),
          });
        }
        return newMap;
      });
    }
  };

  const handleRemove = (lineItemId: string) => {
    setSelectedProducts((prev) => {
      const newMap = new Map(prev);
      newMap.delete(lineItemId);
      return newMap;
    });
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

    const trimmedIdentifier = temporalIdentifier.trim();

    if (!trimmedIdentifier) {
      setError("El identificador temporal es requerido");
      return;
    }

    if (trimmedIdentifier === TEMPORAL_IDENTIFIER_PREFIX) {
      setError(
        `Completa el identificador después de "${TEMPORAL_IDENTIFIER_PREFIX}" (ej: ${TEMPORAL_IDENTIFIER_PREFIX}12)`
      );
      return;
    }

    if (selectedProducts.size === 0) {
      setError("Por favor selecciona al menos un producto");
      return;
    }

    const pending = Array.from(selectedProducts.values()).filter(
      (p) => isWeighedProduct(p.product) && p.quantity <= 0,
    );
    if (pending.length > 0) {
      setError(
        `Ingresa el peso de: ${pending.map((p) => p.product.name).join(", ")}`,
      );
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const orderProducts: OrderProductItem[] = Array.from(
        selectedProducts.values()
      ).map(({ openBillProductId, product, quantity, notes }) => ({
        open_bill_product_id: openBillProductId,
        product_id: product.id,
        quantity,
        notes: notes.trim() || null,
      }));

      await createOrder({
        open_bill_id: openBillId,
        temporal_identifier: temporalIdentifier,
        descriptor: descriptor.trim() || null,
        products: orderProducts,
      });

      onSuccess();
      onClose();
    } catch (err) {
      console.error("Error creating order:", err);
      setError(err instanceof Error ? err.message : "Failed to create order");
    } finally {
      setIsSubmitting(false);
    }
  };

  const selectedProductsArray = Array.from(selectedProducts.values());
  const orderCount = selectedProductsArray.length;
  const qtyInOrderFor = (productId: string) =>
    selectedProductsArray
      .filter((p) => p.product.id === productId)
      .reduce((sum, p) => sum + p.quantity, 0);
  const productInOrder = (productId: string) =>
    selectedProductsArray.some((p) => p.product.id === productId);
  const orderTotal = selectedProductsArray.reduce(
    (sum, { product, quantity }) =>
      sum + parseFloat(product.total_price_with_taxes) * quantity,
    0,
  );

  // Weighed lines with no weight yet block submit — a fish left "pending" would
  // otherwise be charged as 0.
  const pendingWeightNames = selectedProductsArray
    .filter((p) => isWeighedProduct(p.product) && p.quantity <= 0)
    .map((p) => p.product.name);
  const hasPendingWeight = pendingWeightNames.length > 0;

  const canSubmit =
    !isSubmitting && selectedProducts.size > 0 && !hasPendingWeight;

  const pendingWeightHint = hasPendingWeight ? (
    <div
      style={{
        marginBottom: "0.5rem",
        fontSize: "0.8rem",
        fontWeight: 600,
        color: "var(--color-danger)",
        textAlign: "center",
      }}
    >
      Falta el peso de: {pendingWeightNames.join(", ")}
    </div>
  ) : null;

  // Identifier + descriptor fields (shared by both layouts — required for create)
  const orderDetailsFields = (
    <>
      <Input
        label="Identificador (mesa) *"
        type="text"
        value={temporalIdentifier}
        onChange={(e) => setTemporalIdentifier(e.target.value)}
        placeholder="Ej: MESA-12"
        required
      />
      <Textarea
        value={descriptor}
        onChange={(e) => setDescriptor(e.target.value)}
        placeholder="Descriptor (opcional): cliente, mesa..."
        rows={2}
      />
    </>
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
            <h2
              style={{
                margin: 0,
                fontSize: "1.2rem",
                fontWeight: 800,
                letterSpacing: "-0.01em",
                color: "var(--color-text-primary)",
              }}
            >
              Nueva Orden
            </h2>
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
                      active={selectedCategory}
                      onChange={setSelectedCategory}
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
                        inOrder={productInOrder(product.id)}
                        onAdd={handleProductSelect}
                      />
                    ))
                  )}
                </div>
              </>
            ) : (
              <>
                {/* Order details */}
                <div
                  style={{
                    flex: "none",
                    display: "flex",
                    flexDirection: "column",
                    gap: "0.75rem",
                    padding: "0 0.875rem 0.6rem",
                  }}
                >
                  {orderDetailsFields}
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
                        onQuantityChange={handleQuantityChange}
                        onNotesChange={handleNotesChange}
                        onRemove={handleRemove}
                        focusSignal={
                          focusLineItemId === item.lineItemId ? focusNonce : 0
                        }
                      />
                    ))
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
                  {formatCOP(orderTotal)}
                </span>
              </div>
              {pendingWeightHint}
              <Button type="submit" fullWidth size="lg" disabled={!canSubmit} leftIcon={sendIcon}>
                {isSubmitting ? "Creando..." : "Crear Orden"}
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
          <h2
            style={{
              margin: 0,
              fontSize: "1.25rem",
              fontWeight: 800,
              letterSpacing: "-0.01em",
              color: "var(--color-text-primary)",
            }}
          >
            Nueva Orden
          </h2>
        </header>

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
                  active={selectedCategory}
                  onChange={setSelectedCategory}
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
                    inOrder={productInOrder(product.id)}
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
                display: "flex",
                flexDirection: "column",
                gap: "0.75rem",
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
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
              {orderDetailsFields}
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
                    onQuantityChange={handleQuantityChange}
                    onNotesChange={handleNotesChange}
                    onRemove={handleRemove}
                    focusSignal={
                      focusLineItemId === item.lineItemId ? focusNonce : 0
                    }
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
                  {formatCOP(orderTotal)}
                </span>
              </div>
              {pendingWeightHint}
              <Button type="submit" fullWidth size="lg" disabled={!canSubmit} leftIcon={sendIcon}>
                {isSubmitting ? "Creando..." : "Crear Orden"}
              </Button>
            </div>
          </aside>
        </form>
      </div>
    </div>
  );
}
