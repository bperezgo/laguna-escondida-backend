"use client";

import { useState, useEffect, useMemo } from "react";
import { productsApi } from "@/lib/api/products";
import { createOrder } from "@/lib/api/orders";
import type { Product } from "@/types/product";
import type { OrderProductItem } from "@/types/order";

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
  const [temporalIdentifier, setTemporalIdentifier] = useState("MESA-");
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

  // Fetch products on mount
  useEffect(() => {
    const fetchProducts = async () => {
      setIsLoading(true);
      try {
        const fetchedProducts = await productsApi.getAll();
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

    if (!temporalIdentifier.trim()) {
      setError("El identificador temporal es requerido");
      return;
    }

    if (selectedProducts.size === 0) {
      setError("Por favor selecciona al menos un producto");
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
          <h2
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            Crear Nueva Orden
          </h2>
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

            {/* Temporal Identifier */}
            <div style={{ marginBottom: "1rem" }}>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontWeight: "bold",
                  color: "var(--color-text-primary)",
                }}
              >
                Identificador Temporal *
              </label>
              <input
                type="text"
                value={temporalIdentifier}
                onChange={(e) => setTemporalIdentifier(e.target.value)}
                required
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-md)",
                  outline: "none",
                  backgroundColor: "var(--color-bg)",
                  color: "var(--color-text-primary)",
                }}
              />
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
                  Productos Seleccionados ({selectedProductsArray.length})
                </h3>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: "0.75rem",
                  }}
                >
                  {selectedProductsArray.map(
                    ({ lineItemId, product, quantity, notes }) => (
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
                            <strong
                              style={{ color: "var(--color-text-primary)" }}
                            >
                              {product.name}
                            </strong>
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
                                MozAppearance: "textfield",
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
                    )
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
                Agregar Productos
              </h3>
              <div
                style={{
                  display: "flex",
                  gap: "0.75rem",
                  marginBottom: "1rem",
                  flexWrap: "wrap",
                }}
              >
                <input
                  type="text"
                  placeholder="Buscar productos..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  style={{
                    flex: "1 1 250px",
                    padding: "0.75rem",
                    fontSize: "1rem",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-md)",
                    outline: "none",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                  }}
                />
                <select
                  value={selectedCategory}
                  onChange={(e) => setSelectedCategory(e.target.value)}
                  style={{
                    flex: "0 1 200px",
                    padding: "0.75rem",
                    fontSize: "1rem",
                    border: "1px solid var(--color-border)",
                    borderRadius: "var(--radius-md)",
                    outline: "none",
                    backgroundColor: "var(--color-bg)",
                    color: "var(--color-text-primary)",
                    cursor: "pointer",
                  }}
                >
                  <option value="all">Todas las categorías</option>
                  {categories.map((category) => (
                    <option key={category} value={category}>
                      {category}
                    </option>
                  ))}
                </select>
              </div>
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
              disabled={isSubmitting || selectedProducts.size === 0}
              style={{
                padding: "0.75rem 1.5rem",
                fontSize: "1rem",
                fontWeight: "bold",
                backgroundColor:
                  isSubmitting || selectedProducts.size === 0
                    ? "var(--color-text-muted)"
                    : "var(--color-success)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-md)",
                cursor:
                  isSubmitting || selectedProducts.size === 0
                    ? "not-allowed"
                    : "pointer",
              }}
            >
              {isSubmitting ? "Creando..." : "Crear Orden"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
