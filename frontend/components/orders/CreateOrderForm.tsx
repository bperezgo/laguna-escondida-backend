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
  product: Product;
  quantity: number;
  notes: string;
}

export default function CreateOrderForm({
  onClose,
  onSuccess,
}: CreateOrderFormProps) {
  const [temporalIdentifier, setTemporalIdentifier] = useState("");
  const [descriptor, setDescriptor] = useState("");
  const [products, setProducts] = useState<Product[]>([]);
  const [selectedProducts, setSelectedProducts] = useState<
    Map<string, ProductWithQuantity>
  >(new Map());
  const [searchQuery, setSearchQuery] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Generate a simple temporal identifier (can be improved with UUID)
  useEffect(() => {
    // Generate a simple identifier like "TABLE-001" or similar
    const randomNum = Math.floor(Math.random() * 1000)
      .toString()
      .padStart(3, "0");
    setTemporalIdentifier(`TABLE-${randomNum}`);
  }, []);

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

  // Filter products based on search query
  const filteredProducts = useMemo(() => {
    if (!searchQuery.trim()) {
      return products;
    }
    const query = searchQuery.toLowerCase();
    return products.filter(
      (product) =>
        product.name.toLowerCase().includes(query) ||
        product.category.toLowerCase().includes(query)
    );
  }, [products, searchQuery]);

  const handleProductSelect = (product: Product) => {
    setSelectedProducts((prev) => {
      const newMap = new Map(prev);
      if (newMap.has(product.id)) {
        const existing = newMap.get(product.id)!;
        newMap.set(product.id, {
          ...existing,
          quantity: existing.quantity + 1,
        });
      } else {
        newMap.set(product.id, {
          product,
          quantity: 1,
          notes: "",
        });
      }
      return newMap;
    });
  };

  const handleQuantityChange = (productId: string, newQuantity: number) => {
    if (newQuantity <= 0) {
      setSelectedProducts((prev) => {
        const newMap = new Map(prev);
        newMap.delete(productId);
        return newMap;
      });
    } else {
      setSelectedProducts((prev) => {
        const newMap = new Map(prev);
        const existing = newMap.get(productId);
        if (existing) {
          newMap.set(productId, {
            ...existing,
            quantity: newQuantity,
          });
        }
        return newMap;
      });
    }
  };

  const handleNotesChange = (productId: string, notes: string) => {
    setSelectedProducts((prev) => {
      const newMap = new Map(prev);
      const existing = newMap.get(productId);
      if (existing) {
        newMap.set(productId, {
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
      setError("Temporal identifier is required");
      return;
    }

    if (selectedProducts.size === 0) {
      setError("Please select at least one product");
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const orderProducts: OrderProductItem[] = Array.from(
        selectedProducts.values()
      ).map(({ product, quantity, notes }) => ({
        product_id: product.id,
        quantity,
        notes: notes.trim() || null,
      }));

      await createOrder({
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
        backgroundColor: "rgba(0, 0, 0, 0.5)",
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
          backgroundColor: "white",
          borderRadius: "12px",
          maxWidth: "900px",
          width: "100%",
          maxHeight: "90vh",
          overflow: "auto",
          boxShadow: "0 4px 20px rgba(0,0,0,0.2)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div
          style={{
            padding: "1.5rem",
            borderBottom: "2px solid #e0e0e0",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            position: "sticky",
            top: 0,
            backgroundColor: "white",
            zIndex: 1,
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "#333",
            }}
          >
            Create New Order
          </h2>
          <button
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              fontSize: "1.5rem",
              cursor: "pointer",
              color: "#666",
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
                  backgroundColor: "#fee",
                  border: "1px solid #fcc",
                  borderRadius: "8px",
                  color: "#c00",
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
                  color: "#333",
                }}
              >
                Temporal Identifier *
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
                  border: "2px solid #e0e0e0",
                  borderRadius: "8px",
                  outline: "none",
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
                  color: "#333",
                }}
              >
                Descriptor (Optional)
              </label>
              <textarea
                value={descriptor}
                onChange={(e) => setDescriptor(e.target.value)}
                placeholder="Description of the customer or table..."
                rows={3}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "2px solid #e0e0e0",
                  borderRadius: "8px",
                  outline: "none",
                  resize: "vertical",
                  fontFamily: "inherit",
                }}
              />
            </div>

            {/* Selected Products */}
            {selectedProductsArray.length > 0 && (
              <div
                style={{
                  marginBottom: "1.5rem",
                  padding: "1rem",
                  backgroundColor: "#f8f9fa",
                  borderRadius: "8px",
                }}
              >
                <h3
                  style={{
                    margin: "0 0 1rem 0",
                    fontSize: "1.1rem",
                    fontWeight: "bold",
                    color: "#333",
                  }}
                >
                  Selected Products ({selectedProductsArray.length})
                </h3>
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: "0.75rem",
                  }}
                >
                  {selectedProductsArray.map(({ product, quantity, notes }) => (
                    <div
                      key={product.id}
                      style={{
                        padding: "1rem",
                        backgroundColor: "white",
                        borderRadius: "8px",
                        border: "1px solid #e0e0e0",
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
                          <strong style={{ color: "#333" }}>
                            {product.name}
                          </strong>
                          <div style={{ fontSize: "0.875rem", color: "#666" }}>
                            ${product.total_price_with_taxes.toFixed(2)}
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
                              handleQuantityChange(product.id, quantity - 1)
                            }
                            style={{
                              width: "32px",
                              height: "32px",
                              border: "2px solid #007bff",
                              borderRadius: "6px",
                              backgroundColor: "white",
                              color: "#007bff",
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
                          <span
                            style={{
                              minWidth: "40px",
                              textAlign: "center",
                              fontWeight: "bold",
                              fontSize: "1.1rem",
                            }}
                          >
                            {quantity}
                          </span>
                          <button
                            type="button"
                            onClick={() =>
                              handleQuantityChange(product.id, quantity + 1)
                            }
                            style={{
                              width: "32px",
                              height: "32px",
                              border: "2px solid #007bff",
                              borderRadius: "6px",
                              backgroundColor: "white",
                              color: "#007bff",
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
                          handleNotesChange(product.id, e.target.value)
                        }
                        placeholder="Add notes (optional)..."
                        style={{
                          width: "100%",
                          padding: "0.5rem",
                          fontSize: "0.875rem",
                          border: "1px solid #e0e0e0",
                          borderRadius: "6px",
                          outline: "none",
                        }}
                      />
                    </div>
                  ))}
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
                  color: "#333",
                }}
              >
                Add Products
              </h3>
              <input
                type="text"
                placeholder="Search products..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  fontSize: "1rem",
                  border: "2px solid #e0e0e0",
                  borderRadius: "8px",
                  outline: "none",
                  marginBottom: "1rem",
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
                      color: "#666",
                    }}
                  >
                    Loading products...
                  </div>
                ) : filteredProducts.length === 0 ? (
                  <div
                    style={{
                      padding: "2rem",
                      textAlign: "center",
                      color: "#666",
                    }}
                  >
                    No products found
                  </div>
                ) : (
                  filteredProducts.map((product) => {
                    const isSelected = selectedProducts.has(product.id);
                    return (
                      <div
                        key={product.id}
                        onClick={() => handleProductSelect(product)}
                        style={{
                          padding: "1rem",
                          border: `2px solid ${
                            isSelected ? "#007bff" : "#e0e0e0"
                          }`,
                          borderRadius: "8px",
                          cursor: "pointer",
                          backgroundColor: isSelected ? "#e7f3ff" : "white",
                          transition: "all 0.2s",
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.borderColor = "#007bff";
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.borderColor = isSelected
                            ? "#007bff"
                            : "#e0e0e0";
                        }}
                      >
                        <div
                          style={{
                            fontWeight: "bold",
                            color: "#333",
                            marginBottom: "0.25rem",
                            fontSize: "0.95rem",
                          }}
                        >
                          {product.name}
                        </div>
                        <div
                          style={{
                            fontSize: "0.8rem",
                            color: "#666",
                            marginBottom: "0.5rem",
                          }}
                        >
                          {product.category}
                        </div>
                        <div
                          style={{
                            fontSize: "0.9rem",
                            fontWeight: "bold",
                            color: "#28a745",
                          }}
                        >
                          ${product.total_price_with_taxes.toFixed(2)}
                        </div>
                        {isSelected && (
                          <div
                            style={{
                              marginTop: "0.5rem",
                              fontSize: "0.8rem",
                              color: "#007bff",
                              fontWeight: "bold",
                            }}
                          >
                            ✓ Added
                          </div>
                        )}
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
              borderTop: "2px solid #e0e0e0",
              display: "flex",
              gap: "1rem",
              justifyContent: "flex-end",
              position: "sticky",
              bottom: 0,
              backgroundColor: "white",
            }}
          >
            <button
              type="button"
              onClick={onClose}
              style={{
                padding: "0.75rem 1.5rem",
                fontSize: "1rem",
                fontWeight: "bold",
                backgroundColor: "#6c757d",
                color: "white",
                border: "none",
                borderRadius: "8px",
                cursor: "pointer",
              }}
            >
              Cancel
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
                    ? "#6c757d"
                    : "#28a745",
                color: "white",
                border: "none",
                borderRadius: "8px",
                cursor:
                  isSubmitting || selectedProducts.size === 0
                    ? "not-allowed"
                    : "pointer",
              }}
            >
              {isSubmitting ? "Creating..." : "Create Order"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
