import OrganizationBranding from "@/components/layout/OrganizationBranding";

export default function HomeLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <OrganizationBranding>{children}</OrganizationBranding>;
}
