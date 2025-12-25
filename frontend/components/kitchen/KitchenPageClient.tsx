"use client";

import { useState, useEffect, useRef } from "react";
import CommandCard from "./CommandCard";
import { completeCommand } from "@/lib/api/commands";
import type { Command } from "@/types/command";

export default function KitchenPageClient() {
  const [commands, setCommands] = useState<Command[]>([]);
  const [isConnecting, setIsConnecting] = useState(true);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [completingIds, setCompletingIds] = useState<Set<string>>(new Set());
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    let isMounted = true;

    const connectToSSE = () => {
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      setIsConnecting(true);
      setConnectionError(null);

      const eventSource = new EventSource("/api/sse/commands/kitchen");
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        console.log("EventSource connection opened");
        if (isMounted) {
          setIsConnecting(false);
          setConnectionError(null);
        }
      };

      // Handler for pending commands
      const handleCreatedCommand = (event: MessageEvent) => {
        try {
          console.log("command.pending event:", event.data);
          const command: Command = JSON.parse(event.data);

          setCommands((prev) => {
            const existingIndex = prev.findIndex(
              (cmd) => cmd.id === command.id
            );

            let updated: Command[];
            if (existingIndex >= 0) {
              updated = [...prev];
              updated[existingIndex] = command;
            } else {
              updated = [...prev, command];
              updated.sort(
                (a, b) =>
                  new Date(a.created_at).getTime() -
                  new Date(b.created_at).getTime()
              );
            }
            return updated;
          });
        } catch (error) {
          console.error("Error parsing command.pending event:", error);
        }
      };

      // Handler for completed commands
      const handleCompletedCommand = (event: MessageEvent) => {
        try {
          const command: Command = JSON.parse(event.data);
          setCommands((prev) => prev.filter((cmd) => cmd.id !== command.id));
        } catch (error) {
          console.error("Error parsing command.completed event:", error);
        }
      };

      // Handler for cancelled commands
      const handleCancelledCommand = (event: MessageEvent) => {
        try {
          const command: Command = JSON.parse(event.data);
          setCommands((prev) => prev.filter((cmd) => cmd.id !== command.id));
        } catch (error) {
          console.error("Error parsing command.cancelled event:", error);
        }
      };

      eventSource.addEventListener("command.created", handleCreatedCommand);
      eventSource.addEventListener("command.completed", handleCompletedCommand);
      eventSource.addEventListener("command.cancelled", handleCancelledCommand);

      eventSource.onerror = () => {
        eventSource.close();

        if (isMounted) {
          setIsConnecting(false);
          setConnectionError("Conexión perdida. Reconectando...");

          reconnectTimeoutRef.current = setTimeout(() => {
            connectToSSE();
          }, 3000);
        }
      };
    };

    connectToSSE();

    return () => {
      console.log("Cleaning up EventSource...");
      isMounted = false;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, []);

  const handleComplete = async (commandId: string) => {
    setCompletingIds((prev) => new Set(prev).add(commandId));

    try {
      await completeCommand(commandId);
      setCommands((prev) => prev.filter((cmd) => cmd.id !== commandId));
    } catch (error) {
      console.error("Error completing command:", error);
      setConnectionError(
        error instanceof Error
          ? error.message
          : "Error al marcar como completado"
      );
      setTimeout(() => setConnectionError(null), 3000);
    } finally {
      setCompletingIds((prev) => {
        const updated = new Set(prev);
        updated.delete(commandId);
        return updated;
      });
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "1rem",
      }}
    >
      <div
        style={{
          maxWidth: "1600px",
          margin: "0 auto",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1.5rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
            <h1
              style={{
                fontSize: "clamp(1.75rem, 4vw, 2.5rem)",
                fontWeight: "bold",
                margin: 0,
                color: "var(--color-text-primary)",
              }}
            >
              🍳 Comandas de Cocina
            </h1>

            {/* Connection status indicator */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                padding: "0.5rem 1rem",
                borderRadius: "var(--radius-md)",
                backgroundColor: isConnecting
                  ? "var(--color-warning-light)"
                  : connectionError
                  ? "var(--color-danger-light)"
                  : "var(--color-success-light)",
                fontSize: "0.875rem",
                fontWeight: "500",
              }}
            >
              <div
                style={{
                  width: "8px",
                  height: "8px",
                  borderRadius: "50%",
                  backgroundColor: isConnecting
                    ? "var(--color-warning)"
                    : connectionError
                    ? "var(--color-danger)"
                    : "var(--color-success)",
                  animation: isConnecting ? "pulse 1.5s infinite" : "none",
                }}
              />
              <span
                style={{
                  color: isConnecting
                    ? "var(--color-warning)"
                    : connectionError
                    ? "var(--color-danger)"
                    : "var(--color-success)",
                }}
              >
                {isConnecting
                  ? "Conectando..."
                  : connectionError
                  ? connectionError
                  : "En vivo"}
              </span>
            </div>
          </div>

          {/* Commands count */}
          <div
            style={{
              padding: "0.75rem 1.25rem",
              borderRadius: "var(--radius-md)",
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              fontSize: "1.1rem",
              fontWeight: "600",
              color: "var(--color-text-primary)",
            }}
          >
            {commands.length} {commands.length === 1 ? "comanda" : "comandas"}{" "}
            pendiente{commands.length !== 1 ? "s" : ""}
          </div>
        </div>

        {/* Loading State */}
        {isConnecting && commands.length === 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "4rem",
              color: "var(--color-text-secondary)",
              fontSize: "1.1rem",
            }}
          >
            <div
              style={{
                display: "inline-block",
                width: "50px",
                height: "50px",
                border: "4px solid var(--color-border)",
                borderTop: "4px solid var(--color-primary)",
                borderRadius: "50%",
                animation: "spin 1s linear infinite",
                marginBottom: "1rem",
              }}
            />
            <div>Conectando a comandas de cocina...</div>
          </div>
        )}

        {/* Empty State */}
        {!isConnecting && commands.length === 0 && (
          <div
            style={{
              textAlign: "center",
              padding: "4rem",
              backgroundColor: "var(--color-surface)",
              borderRadius: "var(--radius-lg)",
              border: "2px dashed var(--color-border)",
            }}
          >
            <div
              style={{
                fontSize: "4rem",
                marginBottom: "1rem",
              }}
            >
              ✨
            </div>
            <h3
              style={{
                margin: "0 0 0.5rem 0",
                fontSize: "1.5rem",
                color: "var(--color-text-primary)",
              }}
            >
              ¡Todo al día!
            </h3>
            <p
              style={{
                margin: 0,
                color: "var(--color-text-secondary)",
                fontSize: "1.1rem",
              }}
            >
              No hay comandas pendientes en este momento
            </p>
          </div>
        )}

        {/* Commands Grid */}
        {commands.length > 0 && (
          <div
            style={{
              display: "grid",
              gridTemplateColumns:
                "repeat(auto-fill, minmax(min(380px, 100%), 1fr))",
              gap: "1.25rem",
            }}
          >
            {commands.map((command) => (
              <CommandCard
                key={command.id}
                command={command}
                onComplete={handleComplete}
                isCompleting={completingIds.has(command.id)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Pulse animation for connection indicator */}
      <style jsx global>{`
        @keyframes pulse {
          0%,
          100% {
            opacity: 1;
          }
          50% {
            opacity: 0.4;
          }
        }
      `}</style>
    </div>
  );
}
