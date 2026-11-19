import { buildSiteConfig, whatsappLink } from "./build-site-config.ts";

export const site = buildSiteConfig(import.meta.env);

export const whatsappHref = whatsappLink(site, `Hola, quiero ver una demo de ${site.productName} para mi restaurante.`);
