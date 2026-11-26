import AppShell from "@/components/layout/AppShell";
import OrganizationBranding from "@/components/layout/OrganizationBranding";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <OrganizationBranding>
      <AppShell>{children}</AppShell>
    </OrganizationBranding>
  );
}
