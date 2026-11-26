import {
  Bell,
  CalendarCheck,
  CashRegister,
  ChartLineUp,
  CookingPot,
  DesktopTower,
  DotsThree,
  FileText,
  ForkKnife,
  Gear,
  House,
  Invoice,
  Moped,
  Package,
  Plugs,
  Receipt,
  ShoppingCart,
  Stack,
  Star,
  Truck,
  Users,
  Wallet,
  type Icon,
} from "@phosphor-icons/react";
import type { NavIcon as NavIconKey } from "@/lib/home/navItems";

const ICONS: Record<NavIconKey, Icon> = {
  house: House,
  receipt: Receipt,
  "cooking-pot": CookingPot,
  moped: Moped,
  package: Package,
  "calendar-check": CalendarCheck,
  "fork-knife": ForkKnife,
  stack: Stack,
  truck: Truck,
  "shopping-cart": ShoppingCart,
  "chart-line-up": ChartLineUp,
  "cash-register": CashRegister,
  wallet: Wallet,
  invoice: Invoice,
  "file-text": FileText,
  star: Star,
  users: Users,
  gear: Gear,
  plugs: Plugs,
  bell: Bell,
  "desktop-tower": DesktopTower,
  "dots-three": DotsThree,
};

export default function NavIcon({ name, size = 20 }: { name: NavIconKey; size?: number }) {
  const Component = ICONS[name];
  return <Component size={size} aria-hidden="true" />;
}
