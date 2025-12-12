"use client";

import { OpenBill } from "@/types/order";

interface OpenBillCardProps {
  openBill: OpenBill;
  onClick?: () => void;
  onPayClick?: () => void;
}

export default function OpenBillCard({
  openBill,
  onClick,
  onPayClick,
}: OpenBillCardProps) {
  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <div
      onClick={onClick}
      style={{
        backgroundColor: "white",
        border: "2px solid #e0e0e0",
        borderRadius: "12px",
        padding: "1.25rem",
        cursor: onClick ? "pointer" : "default",
        transition: "all 0.2s",
        boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
      }}
      onMouseEnter={(e) => {
        if (onClick) {
          e.currentTarget.style.transform = "translateY(-2px)";
          e.currentTarget.style.boxShadow = "0 4px 12px rgba(0,0,0,0.15)";
          e.currentTarget.style.borderColor = "#007bff";
        }
      }}
      onMouseLeave={(e) => {
        if (onClick) {
          e.currentTarget.style.transform = "translateY(0)";
          e.currentTarget.style.boxShadow = "0 2px 4px rgba(0,0,0,0.1)";
          e.currentTarget.style.borderColor = "#e0e0e0";
        }
      }}
    >
      {/* Temporal Identifier - Main Title */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          marginBottom: "0.75rem",
        }}
      >
        <span
          style={{
            fontSize: "0.75rem",
            fontWeight: "bold",
            color: "#007bff",
            backgroundColor: "#e7f3ff",
            padding: "0.25rem 0.75rem",
            borderRadius: "6px",
            textTransform: "uppercase",
            letterSpacing: "0.5px",
          }}
        >
          {openBill.temporal_identifier}
        </span>
      </div>

      {/* Descriptor */}
      <div
        style={{
          marginBottom: "1rem",
        }}
      >
        {openBill.descriptor ? (
          <p
            style={{
              margin: 0,
              fontSize: "0.95rem",
              color: "#333",
              lineHeight: "1.5",
              display: "-webkit-box",
              WebkitLineClamp: 3,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
              minHeight: "3rem",
            }}
          >
            {openBill.descriptor}
          </p>
        ) : (
          <p
            style={{
              margin: 0,
              fontSize: "0.875rem",
              color: "#999",
              fontStyle: "italic",
              minHeight: "3rem",
            }}
          >
            Sin descripción
          </p>
        )}
      </div>

      {/* Meta Information */}
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
          paddingTop: "0.75rem",
          borderTop: "1px solid #f0f0f0",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
          }}
        >
          <span
            style={{
              fontSize: "0.8rem",
              color: "#666",
              fontWeight: "500",
            }}
          >
            Creado por:
          </span>
          <span
            style={{
              fontSize: "0.8rem",
              color: "#333",
              fontWeight: "bold",
            }}
          >
            {openBill.created_by?.user_name}
          </span>
        </div>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
          }}
        >
          <span
            style={{
              fontSize: "0.8rem",
              color: "#666",
              fontWeight: "500",
            }}
          >
            Creado el:
          </span>
          <span
            style={{
              fontSize: "0.8rem",
              color: "#333",
            }}
          >
            {formatDate(openBill.created_at)}
          </span>
        </div>
      </div>

      {/* Pay Button */}
      {onPayClick && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onPayClick();
          }}
          style={{
            marginTop: "1rem",
            width: "100%",
            padding: "0.75rem",
            fontSize: "1rem",
            fontWeight: "bold",
            backgroundColor: "#28a745",
            color: "white",
            border: "none",
            borderRadius: "8px",
            cursor: "pointer",
            transition: "background-color 0.2s",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = "#218838";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = "#28a745";
          }}
        >
          💳 Pagar Cuenta
        </button>
      )}
    </div>
  );
}
