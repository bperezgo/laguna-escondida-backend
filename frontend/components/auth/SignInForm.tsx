"use client";

import { useState, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { usePermissions } from "@/lib/permissions";

export default function SignInForm() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();
  const { fetchPermissions } = usePermissions();

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");
    setIsLoading(true);

    try {
      const response = await fetch("/api/auth/signin", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, password }),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || "Failed to sign in");
      }

      // Fetch user permissions with the new cookie before navigating
      await fetchPermissions();

      // Redirect to home on success
      router.push("/home");
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: "var(--color-bg)",
        padding: "1rem",
      }}
    >
      <div
        style={{
          maxWidth: "28rem",
          width: "100%",
        }}
      >
        <div>
          <h2
            style={{
              marginTop: "1.5rem",
              textAlign: "center",
              fontSize: "1.875rem",
              fontWeight: "800",
              color: "var(--color-text-primary)",
              marginBottom: "2rem",
            }}
          >
            Inicia sesión en tu cuenta
          </h2>
        </div>
        <form
          onSubmit={handleSubmit}
          style={{
            marginTop: "2rem",
          }}
        >
          <div
            style={{
              borderRadius: "var(--radius-md)",
              boxShadow: "var(--shadow-sm)",
              backgroundColor: "var(--color-surface)",
              border: "1px solid var(--color-border)",
              overflow: "hidden",
            }}
          >
            <div>
              <label
                htmlFor="username"
                style={{
                  position: "absolute",
                  width: "1px",
                  height: "1px",
                  padding: "0",
                  margin: "-1px",
                  overflow: "hidden",
                  clip: "rect(0, 0, 0, 0)",
                  whiteSpace: "nowrap",
                  borderWidth: "0",
                }}
              >
                Usuario
              </label>
              <input
                id="username"
                name="username"
                type="text"
                required
                placeholder="Usuario"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={isLoading}
                style={{
                  appearance: "none",
                  position: "relative",
                  display: "block",
                  width: "100%",
                  padding: "0.875rem 1rem",
                  border: "none",
                  borderBottom: "1px solid var(--color-border)",
                  color: "var(--color-text-primary)",
                  fontSize: "1rem",
                  outline: "none",
                  backgroundColor: "transparent",
                }}
              />
            </div>
            <div>
              <label
                htmlFor="password"
                style={{
                  position: "absolute",
                  width: "1px",
                  height: "1px",
                  padding: "0",
                  margin: "-1px",
                  overflow: "hidden",
                  clip: "rect(0, 0, 0, 0)",
                  whiteSpace: "nowrap",
                  borderWidth: "0",
                }}
              >
                Contraseña
              </label>
              <input
                id="password"
                name="password"
                type="password"
                required
                placeholder="Contraseña"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoading}
                style={{
                  appearance: "none",
                  position: "relative",
                  display: "block",
                  width: "100%",
                  padding: "0.875rem 1rem",
                  border: "none",
                  color: "var(--color-text-primary)",
                  fontSize: "1rem",
                  outline: "none",
                  backgroundColor: "transparent",
                }}
              />
            </div>
          </div>

          {error && (
            <div
              style={{
                marginTop: "1.5rem",
                borderRadius: "var(--radius-md)",
                backgroundColor: "var(--color-danger-light)",
                padding: "1rem",
                border: "1px solid var(--color-danger)",
              }}
            >
              <div style={{ display: "flex" }}>
                <div style={{ marginLeft: "0.75rem" }}>
                  <h3
                    style={{
                      fontSize: "0.875rem",
                      fontWeight: "500",
                      color: "var(--color-danger)",
                      margin: 0,
                    }}
                  >
                    {error}
                  </h3>
                </div>
              </div>
            </div>
          )}

          <div style={{ marginTop: "1.5rem" }}>
            <button
              type="submit"
              disabled={isLoading}
              style={{
                position: "relative",
                width: "100%",
                display: "flex",
                justifyContent: "center",
                padding: "0.875rem 1rem",
                border: "none",
                fontSize: "1rem",
                fontWeight: "600",
                borderRadius: "var(--radius-md)",
                color: "white",
                backgroundColor: isLoading ? "var(--color-text-muted)" : "var(--color-primary)",
                cursor: isLoading ? "not-allowed" : "pointer",
                opacity: isLoading ? 0.7 : 1,
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                if (!isLoading) {
                  e.currentTarget.style.backgroundColor = "var(--color-primary-hover)";
                }
              }}
              onMouseLeave={(e) => {
                if (!isLoading) {
                  e.currentTarget.style.backgroundColor = "var(--color-primary)";
                }
              }}
            >
              {isLoading ? "Iniciando sesión..." : "Iniciar sesión"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
