import KitchenTabs, { type KitchenView } from "@/components/kitchen/KitchenTabs";
import KitchenCommandView from "@/components/kitchen/KitchenCommandView";
import KitchenCommandItemsView from "@/components/kitchen/KitchenCommandItemsView";
import KitchenReadyView from "@/components/kitchen/KitchenReadyView";

interface KitchenPageProps {
  searchParams: Promise<{ view?: string }>;
}

export default async function KitchenPage({ searchParams }: KitchenPageProps) {
  const params = await searchParams;
  const view: KitchenView =
    params.view === "command_items"
      ? "command_items"
      : params.view === "ready"
      ? "ready"
      : "commands";

  return (
    // Kitchen/comandas is a glanceable display (cooks). Per user preference it now
    // uses the Clean Light theme like the rest of the app (token-driven, so the
    // color-coded countdown badges still pop on the light canvas).
    <div style={{ minHeight: "100vh" }}>
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
      ) : view === "command_items" ? (
        <KitchenCommandItemsView />
      ) : (
        <KitchenReadyView />
      )}
    </div>
  );
}
