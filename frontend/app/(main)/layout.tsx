import PageHeader from "@/components/layout/PageHeader";

export default function MainLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      <PageHeader />
      {children}
    </>
  );
}
