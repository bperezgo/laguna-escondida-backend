import { siteActions } from "./actions.ts";
import { buildSiteConfig, installerDownloadUrl, whatsappLink } from "./build-site-config.ts";

export const site = buildSiteConfig(import.meta.env);

export const whatsappHref = whatsappLink(site, `Hola, quiero ver una demo de ${site.productName} para mi restaurante.`);

export const agentHref = whatsappLink(site, `Hola, quiero saber más del agente de WhatsApp para mi restaurante con ${site.productName}.`);

export const installerHref = installerDownloadUrl(site);

export const actions = siteActions(site);
