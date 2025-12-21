"use client";

import { useState } from "react";
import type { OrderItem } from "@/types/order";

interface Product {
  id: string;
  name: string;
  image?: string;
  price: number;
  vat: number;
}

interface OrderItemCardProps {
  product: Product;
  quantity: number;
  comment?: string;
  onQuantityChange: (productId: string, delta: number) => void;
  onCommentChange: (productId: string, comment: string) => void;
}

export default function OrderItemCard({
  product,
  quantity,
  comment = "",
  onQuantityChange,
  onCommentChange,
}: OrderItemCardProps) {
  const [isCommentExpanded, setIsCommentExpanded] = useState(false);
  const [localComment, setLocalComment] = useState(comment);

  const handleCommentChange = (value: string) => {
    setLocalComment(value);
    onCommentChange(product.id, value);
    // Auto-expand if comment is getting long
    if (value.length > 30 && !isCommentExpanded) {
      setIsCommentExpanded(true);
    }
  };

  const totalPrice =
    (product.price + (product.price * product.vat) / 100) * quantity;

  return (
    <div
      style={{
        display: "flex",
        gap: "0.75rem",
        padding: "0.75rem",
        backgroundColor: "white",
        border: "1px solid #e0e0e0",
        borderRadius: "8px",
        marginBottom: "1rem",
        boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
        flexWrap: "wrap",
      }}
    >
      {/* Product Image */}
      <div
        style={{
          width: "clamp(60px, 15vw, 80px)",
          height: "clamp(60px, 15vw, 80px)",
          minWidth: "clamp(60px, 15vw, 80px)",
          borderRadius: "8px",
          backgroundColor: "#f8f9fa",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          overflow: "hidden",
          border: "1px solid #e0e0e0",
        }}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            style={{
              width: "100%",
              height: "100%",
              objectFit: "cover",
            }}
          />
        ) : (
          <span style={{ color: "#6c757d", fontSize: "2rem" }}>🍽️</span>
        )}
      </div>

      {/* Product Info and Controls */}
      <div
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          gap: "0.75rem",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
          }}
        >
          <div style={{ flex: 1 }}>
            <h3
              style={{
                margin: 0,
                fontSize: "1.1rem",
                fontWeight: "bold",
                color: "#333",
              }}
            >
              {product.name}
            </h3>
            <p
              style={{
                margin: "0.25rem 0 0 0",
                color: "#666",
                fontSize: "0.9rem",
              }}
            >
              ${product.price} + {product.vat}% VAT
            </p>
          </div>
          <div
            style={{ fontSize: "1.1rem", fontWeight: "bold", color: "#28a745" }}
          >
            ${totalPrice.toFixed(2)}
          </div>
        </div>

        {/* Quantity Controls and Comment */}
        <div
          style={{
            display: "flex",
            gap: "0.5rem",
            alignItems: "center",
            flexWrap: "wrap",
            width: "100%",
          }}
        >
          {/* Decrease Button */}
          <button
            onClick={() => onQuantityChange(product.id, -1)}
            disabled={quantity <= 0}
            style={{
              width: "36px",
              height: "36px",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              backgroundColor: quantity <= 0 ? "#e9ecef" : "#dc3545",
              color: "white",
              border: "none",
              borderRadius: "6px",
              cursor: quantity <= 0 ? "not-allowed" : "pointer",
              fontSize: "1.2rem",
              fontWeight: "bold",
              transition: "background-color 0.2s",
            }}
            onMouseEnter={(e) => {
              if (quantity > 0) {
                e.currentTarget.style.backgroundColor = "#c82333";
              }
            }}
            onMouseLeave={(e) => {
              if (quantity > 0) {
                e.currentTarget.style.backgroundColor = "#dc3545";
              }
            }}
          >
            −
          </button>

          {/* Quantity Display */}
          <div
            style={{
              minWidth: "40px",
              textAlign: "center",
              fontSize: "1.1rem",
              fontWeight: "bold",
              color: "#333",
            }}
          >
            {quantity}
          </div>

          {/* Increase Button */}
          <button
            onClick={() => onQuantityChange(product.id, 1)}
            style={{
              width: "36px",
              height: "36px",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              backgroundColor: "#28a745",
              color: "white",
              border: "none",
              borderRadius: "6px",
              cursor: "pointer",
              fontSize: "1.2rem",
              fontWeight: "bold",
              transition: "background-color 0.2s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#218838";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#28a745";
            }}
          >
            +
          </button>

          {/* Comment Input */}
          <div
            style={{
              flex: "1 1 200px",
              minWidth: "150px",
              position: "relative",
            }}
          >
            <input
              type="text"
              value={localComment}
              onChange={(e) => handleCommentChange(e.target.value)}
              placeholder="Agregar comentario..."
              style={{
                width: "100%",
                padding: "0.5rem",
                fontSize: "0.875rem",
                border: "1px solid #ced4da",
                borderRadius: "4px",
                outline: "none",
                transition: "border-color 0.2s",
              }}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = "#007bff";
                setIsCommentExpanded(true);
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = "#ced4da";
                if (!localComment) {
                  setIsCommentExpanded(false);
                }
              }}
            />
            {isCommentExpanded && (
              <textarea
                value={localComment}
                onChange={(e) => handleCommentChange(e.target.value)}
                placeholder="Agregar comentario detallado..."
                style={{
                  width: "100%",
                  minHeight: "60px",
                  padding: "0.5rem",
                  fontSize: "0.875rem",
                  border: "1px solid #ced4da",
                  borderRadius: "4px",
                  outline: "none",
                  marginTop: "0.25rem",
                  resize: "vertical",
                  fontFamily: "inherit",
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = "#007bff";
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = "#ced4da";
                }}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
