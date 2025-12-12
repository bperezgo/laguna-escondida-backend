"use client";

import { useState, FormEvent } from "react";
import { useRouter } from "next/navigation";

export default function SignInForm() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

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

      // Redirect to home on success
      router.push("/home");
      router.refresh();
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
        backgroundColor: "#f9fafb",
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
              color: "#111827",
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
              borderRadius: "0.375rem",
              boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.05)",
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
                  padding: "0.625rem 0.75rem",
                  border: "1px solid #d1d5db",
                  borderRadius: "0.375rem 0.375rem 0 0",
                  color: "#111827",
                  fontSize: "0.875rem",
                  outline: "none",
                  backgroundColor: "white",
                }}
              />
            </div>
            <div style={{ marginTop: "-1px" }}>
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
                  padding: "0.625rem 0.75rem",
                  border: "1px solid #d1d5db",
                  borderRadius: "0 0 0.375rem 0.375rem",
                  color: "#111827",
                  fontSize: "0.875rem",
                  outline: "none",
                  backgroundColor: "white",
                }}
              />
            </div>
          </div>

          {error && (
            <div
              style={{
                marginTop: "1.5rem",
                borderRadius: "0.375rem",
                backgroundColor: "#fef2f2",
                padding: "1rem",
              }}
            >
              <div style={{ display: "flex" }}>
                <div style={{ marginLeft: "0.75rem" }}>
                  <h3
                    style={{
                      fontSize: "0.875rem",
                      fontWeight: "500",
                      color: "#991b1b",
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
                padding: "0.625rem 1rem",
                border: "none",
                fontSize: "0.875rem",
                fontWeight: "500",
                borderRadius: "0.375rem",
                color: "white",
                backgroundColor: isLoading ? "#9ca3af" : "#4f46e5",
                cursor: isLoading ? "not-allowed" : "pointer",
                opacity: isLoading ? 0.5 : 1,
              }}
              onMouseEnter={(e) => {
                if (!isLoading) {
                  e.currentTarget.style.backgroundColor = "#4338ca";
                }
              }}
              onMouseLeave={(e) => {
                if (!isLoading) {
                  e.currentTarget.style.backgroundColor = "#4f46e5";
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
