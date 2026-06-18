"use client";

import { useState, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { usePermissions } from "@/lib/permissions";
import { Card, CardBody, Input, Button } from "@/components/ui";

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
      <div style={{ maxWidth: "26rem", width: "100%" }}>
        {/* Brand */}
        <div style={{ textAlign: "center", marginBottom: "1.75rem" }}>
          <h1
            style={{
              fontSize: "1.75rem",
              fontWeight: 800,
              letterSpacing: "-0.02em",
              color: "var(--color-primary)",
              margin: 0,
            }}
          >
            Laguna Escondida
          </h1>
          <p
            style={{
              fontSize: "0.95rem",
              color: "var(--color-text-secondary)",
              marginTop: "0.5rem",
            }}
          >
            Inicia sesión para continuar
          </p>
        </div>

        <Card>
          <CardBody>
            <form
              onSubmit={handleSubmit}
              style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
            >
              <Input
                label="Usuario"
                id="username"
                name="username"
                type="text"
                required
                placeholder="Ingresa tu usuario"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={isLoading}
              />

              <Input
                label="Contraseña"
                id="password"
                name="password"
                type="password"
                required
                placeholder="Ingresa tu contraseña"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoading}
              />

              {error && (
                <div
                  style={{
                    borderRadius: "var(--radius-md)",
                    backgroundColor: "var(--color-danger-light)",
                    border: "1px solid var(--color-danger)",
                    color: "var(--color-danger)",
                    padding: "0.75rem 1rem",
                    fontSize: "0.875rem",
                    fontWeight: 500,
                  }}
                >
                  {error}
                </div>
              )}

              <Button
                type="submit"
                variant="primary"
                fullWidth
                disabled={isLoading}
              >
                {isLoading ? "Iniciando sesión..." : "Iniciar sesión"}
              </Button>
            </form>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
