export interface Faq {
  question: string;
  answer: string;
}

export const faqs: Faq[] = [
  {
    question: "¿Qué pasa si se cae el internet en medio del servicio?",
    answer:
      "Sigues cobrando y la cocina sigue recibiendo pedidos, porque el sistema corre en un equipo dentro de tu local. Lo que no necesita conexión no se detiene, y todo se envía a la nube cuando vuelve el internet. Los pedidos por WhatsApp sí necesitan conexión para llegar.",
  },
  {
    question: "¿La factura es válida ante la DIAN?",
    answer:
      "El sistema emite facturas electrónicas y documentos soporte con tus prefijos de facturación. Los dejamos configurados contigo durante la puesta en marcha.",
  },
  {
    question: "¿Es difícil cambiar de sistema?",
    answer:
      "No empiezas solo. Instalamos el equipo, cargamos tu menú y dejamos listo tu canal de WhatsApp antes de que abras con el sistema nuevo.",
  },
  {
    question: "¿Mi equipo va a aprender a usarlo?",
    answer:
      "Cada persona ve solo lo que necesita: caja, cocina o administración. Con la demo puedes ver las pantallas que usaría tu equipo antes de decidir.",
  },
  {
    question: "¿Funciona para domicilios y para llevar?",
    answer:
      "Sí. Defines tus zonas de domicilio en un mapa, y puedes pausar domicilios o pedidos para llevar y fijar sus horarios.",
  },
  {
    question: "¿Cuánto cuesta?",
    answer: "En la demo te explicamos las condiciones según el tamaño de tu restaurante. No publicamos precios en esta página.",
  },
];
