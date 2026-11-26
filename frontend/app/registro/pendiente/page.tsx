import { Card, CardBody } from "@/components/ui";
import PublicShell from "@/components/auth/PublicShell";
import { supportEmail, supportMailto } from "@/lib/support";

export default function SignupPendingPage() {
  const email = supportEmail();
  const mailto = supportMailto(email, "Solicitud de prueba");
  return (
    <PublicShell subtitle="Solicitud recibida">
      <Card>
        <CardBody>
          <div style={{ display: "flex", flexDirection: "column", gap: "1rem", textAlign: "center" }}>
            <h1 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>Solicitud recibida</h1>
            <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
              Recibimos la solicitud de tu restaurante. Un operador aprobará tu suscripción antes de que empiecen tus
              14 días de prueba. Cuando esté aprobada, podrás iniciar sesión con tu cuenta de Google.
            </p>
            {email && mailto && (
              <p style={{ margin: 0, color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
                ¿Preguntas? Escríbenos a{" "}
                <a href={mailto} style={{ color: "var(--color-primary)" }}>
                  {email}
                </a>
              </p>
            )}
            <p style={{ margin: 0, fontSize: "0.875rem" }}>
              <a href="/signin" style={{ color: "var(--color-primary)" }}>
                Volver a iniciar sesión
              </a>
            </p>
          </div>
        </CardBody>
      </Card>
    </PublicShell>
  );
}
