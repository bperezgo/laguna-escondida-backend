import { formatCop, plans } from "../../config/plans.ts";

export interface Faq {
  question: string;
  answer: string;
}

const prices = plans.map((plan) => `${plan.name} a ${formatCop(plan.monthly)}`);

export const faqs: Faq[] = [
  {
    question: "¿Qué pasa si se cae el internet en medio del servicio?",
    answer:
      "Sigues cobrando y la cocina sigue recibiendo pedidos, porque el sistema corre en un equipo dentro de tu local. Lo que no necesita conexión no se detiene, y todo se envía a la nube cuando vuelve el internet. Los pedidos que llegan por internet sí necesitan conexión.",
  },
  {
    question: "¿La factura es válida ante la DIAN?",
    answer:
      "El sistema emite facturas electrónicas y documentos soporte con tus prefijos de facturación. Están en los planes Facturación electrónica y Completo.",
  },
  {
    question: "¿Es difícil cambiar de sistema?",
    answer:
      "No empiezas solo. Te acompañamos a cargar tu menú antes de que abras con el sistema nuevo. Tú instalas el sistema en el computador del local con nuestro instalador.",
  },
  {
    question: "¿Mi equipo va a aprender a usarlo?",
    answer:
      "Cada persona ve solo lo que necesita: caja, cocina o administración. Con la demo puedes ver las pantallas que usaría tu equipo antes de decidir.",
  },
  {
    question: "¿Funciona para domicilios y para llevar?",
    answer:
      "Sí. Los pedidos para llevar están en todos los planes. Con el plan Completo defines tus zonas de domicilio en un mapa, y puedes pausar domicilios o pedidos para llevar y fijar sus horarios.",
  },
  {
    question: "¿El agente de WhatsApp viene incluido?",
    answer:
      "No. Es un producto aparte que se conecta con el sistema y se contrata por separado. El plan Completo permite esa conexión. Escríbenos y te contamos cómo funciona.",
  },
  {
    question: "¿Cuánto cuesta?",
    answer: `Hay tres planes, con un precio fijo al mes en pesos colombianos: ${prices[0]}, ${prices[1]} y ${prices[2]}. Si pagas el año, dos meses son gratis. El detalle de cada plan está en la sección de planes.`,
  },
];
