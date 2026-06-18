"use client";

import { useState } from "react";

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

  const stepperBtn = (disabled: boolean): React.CSSProperties => ({
    width: "44px",
    height: "44px",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    background: "var(--color-surface)",
    border: "none",
    fontSize: "1.4rem",
    fontWeight: 700,
    lineHeight: 1,
    cursor: disabled ? "not-allowed" : "pointer",
    color: disabled ? "var(--color-text-muted)" : "var(--color-text-primary)",
    transition: "background-color var(--transition-fast)",
  });

  const commentInputStyle: React.CSSProperties = {
    width: "100%",
    padding: "0.6rem 0.75rem",
    fontSize: "0.875rem",
    fontFamily: "inherit",
    color: "var(--color-text-primary)",
    background: "var(--color-surface)",
    border: "1px solid var(--color-border-strong)",
    borderRadius: "var(--radius-sm)",
    outline: "none",
    transition: "border-color var(--transition-fast), box-shadow var(--transition-fast)",
  };

  const focusOn = (el: HTMLInputElement | HTMLTextAreaElement) => {
    el.style.borderColor = "var(--color-primary)";
    el.style.boxShadow = "0 0 0 3px var(--color-primary-light)";
  };
  const focusOff = (el: HTMLInputElement | HTMLTextAreaElement) => {
    el.style.borderColor = "var(--color-border-strong)";
    el.style.boxShadow = "none";
  };

  return (
    <div
      style={{
        display: "flex",
        gap: "0.75rem",
        padding: "0.75rem",
        background: "var(--color-surface)",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        marginBottom: "1rem",
        boxShadow: "var(--shadow-sm)",
        flexWrap: "wrap",
      }}
    >
      {/* Product Image */}
      <div
        style={{
          width: "clamp(60px, 15vw, 80px)",
          height: "clamp(60px, 15vw, 80px)",
          minWidth: "clamp(60px, 15vw, 80px)",
          borderRadius: "var(--radius-sm)",
          background: "var(--color-neutral-bg)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          overflow: "hidden",
          border: "1px solid var(--color-border)",
        }}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            style={{ width: "100%", height: "100%", objectFit: "cover" }}
          />
        ) : (
          <span style={{ fontSize: "2rem" }}>🍽️</span>
        )}
      </div>

      {/* Product Info and Controls */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: "0.75rem", minWidth: 0 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: "0.75rem" }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <h3
              style={{
                margin: 0,
                fontSize: "1.05rem",
                fontWeight: 700,
                color: "var(--color-text-primary)",
              }}
            >
              {product.name}
            </h3>
            <p style={{ margin: "0.25rem 0 0 0", color: "var(--color-text-secondary)", fontSize: "0.85rem" }}>
              ${product.price} + {product.vat}% IVA
            </p>
          </div>
          <div
            style={{
              fontSize: "1.1rem",
              fontWeight: 800,
              color: "var(--color-text-primary)",
              fontVariantNumeric: "tabular-nums",
              whiteSpace: "nowrap",
            }}
          >
            ${totalPrice.toFixed(2)}
          </div>
        </div>

        {/* Quantity Controls and Comment */}
        <div style={{ display: "flex", gap: "0.75rem", alignItems: "center", flexWrap: "wrap", width: "100%" }}>
          {/* Stepper */}
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              border: "1px solid var(--color-border-strong)",
              borderRadius: "var(--radius-sm)",
              overflow: "hidden",
              background: "var(--color-surface)",
            }}
          >
            <button
              type="button"
              onClick={() => onQuantityChange(product.id, -1)}
              disabled={quantity <= 0}
              aria-label="Disminuir cantidad"
              style={{
                ...stepperBtn(quantity <= 0),
                borderRight: "1px solid var(--color-border)",
                color: quantity <= 0 ? "var(--color-text-muted)" : "var(--color-danger)",
              }}
            >
              −
            </button>
            <div
              style={{
                minWidth: "48px",
                textAlign: "center",
                fontSize: "1.05rem",
                fontWeight: 700,
                color: "var(--color-text-primary)",
                fontVariantNumeric: "tabular-nums",
              }}
            >
              {quantity}
            </div>
            <button
              type="button"
              onClick={() => onQuantityChange(product.id, 1)}
              aria-label="Aumentar cantidad"
              style={{
                ...stepperBtn(false),
                borderLeft: "1px solid var(--color-border)",
                color: "var(--color-primary)",
              }}
            >
              +
            </button>
          </div>

          {/* Comment Input */}
          <div style={{ flex: "1 1 200px", minWidth: "150px", position: "relative" }}>
            <input
              type="text"
              value={localComment}
              onChange={(e) => handleCommentChange(e.target.value)}
              placeholder="Agregar comentario..."
              style={commentInputStyle}
              onFocus={(e) => {
                focusOn(e.currentTarget);
                setIsCommentExpanded(true);
              }}
              onBlur={(e) => {
                focusOff(e.currentTarget);
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
                  ...commentInputStyle,
                  minHeight: "60px",
                  marginTop: "0.5rem",
                  resize: "vertical",
                }}
                onFocus={(e) => focusOn(e.currentTarget)}
                onBlur={(e) => focusOff(e.currentTarget)}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
