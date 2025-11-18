export default function RootPage() {
  // This page is a placeholder - middleware will redirect users to:
  // - /signin if not authenticated
  // - /home if authenticated
  return (
    <div className="min-h-screen flex items-center justify-center">
      <p>Redirecting...</p>
    </div>
  );
}

