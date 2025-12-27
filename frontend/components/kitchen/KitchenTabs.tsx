"use client";

import { useRouter } from "next/navigation";

interface KitchenTabsProps {
  currentView: "commands" | "command_items";
}

export default function KitchenTabs({ currentView }: KitchenTabsProps) {
  const router = useRouter();

  const handleTabChange = (newView: "commands" | "command_items") => {
    if (newView === currentView) return;
    const params = new URLSearchParams();
    params.set("view", newView);
    router.push(`/kitchen?${params.toString()}`);
  };

  return (
    <div
      style={{
        display: "flex",
        gap: "0.5rem",
        marginBottom: "1.5rem",
        borderBottom: "2px solid var(--color-border)",
        paddingBottom: "0",
      }}
    >
      <button
        onClick={() => handleTabChange("commands")}
        style={{
          padding: "0.75rem 1.5rem",
          fontSize: "1rem",
          fontWeight: "600",
          backgroundColor: "transparent",
          color:
            currentView === "commands"
              ? "var(--color-primary)"
              : "var(--color-text-secondary)",
          border: "none",
          borderBottom:
            currentView === "commands"
              ? "3px solid var(--color-primary)"
              : "3px solid transparent",
          cursor: "pointer",
          transition: "all 0.2s ease",
          marginBottom: "-2px",
        }}
      >
        Comandas
      </button>
      <button
        onClick={() => handleTabChange("command_items")}
        style={{
          padding: "0.75rem 1.5rem",
          fontSize: "1rem",
          fontWeight: "600",
          backgroundColor: "transparent",
          color:
            currentView === "command_items"
              ? "var(--color-primary)"
              : "var(--color-text-secondary)",
          border: "none",
          borderBottom:
            currentView === "command_items"
              ? "3px solid var(--color-primary)"
              : "3px solid transparent",
          cursor: "pointer",
          transition: "all 0.2s ease",
          marginBottom: "-2px",
        }}
      >
        Comandas Individuales
      </button>
    </div>
  );
}

