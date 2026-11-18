import PageHeader from "@/components/layout/PageHeader";
import OrganizationBranding from "@/components/layout/OrganizationBranding";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <OrganizationBranding>
      <PageHeader />
      {children}
    </OrganizationBranding>
  );
}
