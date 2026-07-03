"use client";

import { useRouter } from "next/navigation";

export type KitchenView = "commands" | "command_items" | "ready";

interface KitchenTabsProps {
  currentView: KitchenView;
}

const TABS: { view: KitchenView; label: string }[] = [
  { view: "commands", label: "Comandas" },
  { view: "command_items", label: "Comandas Individuales" },
  { view: "ready", label: "Comandas Listas" },
];

export default function KitchenTabs({ currentView }: KitchenTabsProps) {
  const router = useRouter();

  const handleTabChange = (newView: KitchenView) => {
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
      {TABS.map((tab) => {
        const active = currentView === tab.view;
        return (
          <button
            key={tab.view}
            onClick={() => handleTabChange(tab.view)}
            style={{
              padding: "0.75rem 1.5rem",
              fontSize: "1rem",
              fontWeight: "600",
              backgroundColor: "transparent",
              color: active
                ? "var(--color-primary)"
                : "var(--color-text-secondary)",
              border: "none",
              borderBottom: active
                ? "3px solid var(--color-primary)"
                : "3px solid transparent",
              cursor: "pointer",
              transition: "all 0.2s ease",
              marginBottom: "-2px",
            }}
          >
            {tab.label}
          </button>
        );
      })}
    </div>
  );
}
