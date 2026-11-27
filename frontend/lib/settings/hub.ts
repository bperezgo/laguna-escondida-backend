import { PERMISSIONS } from "../permissions/constants.ts";
import { pageVisible } from "../plans.ts";

export interface SettingsSection {
  href: string;
  title: string;
  description: string;
  permission: string;
}

export const SETTINGS_SECTIONS: readonly SettingsSection[] = [
  {
    href: "/ajustes/integraciones",
    title: "Integraciones",
    description: "Servicios externos del restaurante, como las llaves de Wompi.",
    permission: PERMISSIONS.INTEGRATIONS_READ,
  },
  {
    href: "/ajustes/notificaciones",
    title: "Notificaciones",
    description: "Avisos al personal en Discord: pedidos pagados, reservas por confirmar y más.",
    permission: PERMISSIONS.NOTIFICATION_CHANNELS_READ,
  },
  {
    href: "/ajustes/instalar-caja",
    title: "Instalar caja",
    description: "Descarga el instalador, genera el código de instalación y la contraseña de la caja.",
    permission: PERMISSIONS.EDGE_ENROLL,
  },
  {
    href: "/ajustes/suscripcion",
    title: "Suscripción",
    description: "Tu plan, el próximo cobro, la tarjeta guardada y los cobros hechos.",
    permission: PERMISSIONS.BILLING_MANAGE,
  },
];

export interface VisibilityContext {
  hasPermission: (permission: string) => boolean;
  plan: string | null | undefined;
  isEdge: boolean;
}

export function visibleSettingsSections({ hasPermission, plan, isEdge }: VisibilityContext): SettingsSection[] {
  return SETTINGS_SECTIONS.filter((s) => hasPermission(s.permission) && pageVisible(s.href, plan, isEdge));
}
