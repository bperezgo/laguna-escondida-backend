import { redirect } from "next/navigation";

// The zones moved into the Domicilios page; old bookmarks land on its Zonas tab.
export default function DeliveryZonesPage() {
  redirect("/deliveries?tab=zonas");
}
