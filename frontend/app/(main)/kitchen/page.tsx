import KitchenTabs from "@/components/kitchen/KitchenTabs";
import KitchenCommandView from "@/components/kitchen/KitchenCommandView";
import KitchenCommandItemsView from "@/components/kitchen/KitchenCommandItemsView";

interface KitchenPageProps {
  searchParams: Promise<{ view?: string }>;
}

export default async function KitchenPage({ searchParams }: KitchenPageProps) {
  const params = await searchParams;
  const view = params.view === "command_items" ? "command_items" : "commands";

  return (
    // Kitchen/comandas is an always-dark, glanceable display (cooks).
    // data-theme="dark" cascades the "Midnight Waters" palette to the whole subtree.
    <div data-theme="dark" style={{ minHeight: "100vh" }}>
      <div
        style={{
          backgroundColor: "var(--color-bg)",
          padding: "1rem 1rem 0 1rem",
        }}
      >
        <div
          style={{
            maxWidth: "1600px",
            margin: "0 auto",
          }}
        >
          <KitchenTabs currentView={view} />
        </div>
      </div>
      {view === "commands" ? (
        <KitchenCommandView />
      ) : (
        <KitchenCommandItemsView />
      )}
    </div>
  );
}
