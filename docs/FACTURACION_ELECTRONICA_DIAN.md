# Facturación electrónica: LaFactura o conexión directa a la DIAN

Investigación del 2026-11-25. No es una especificación: es el mapa para decidir y, si hace falta, arrancar.

## Resumen

Primero se negocia con LaFactura; la conexión directa a la DIAN queda como plan B y solo para cuando el volumen la pague.

- **La DIAN no cobra** por habilitarse, por usar el web service de validación ni por documento. El costo de ir directo es de desarrollo, certificados y mantenimiento.
- **LaFactura cuesta hoy** 30 pesos por factura, con un mínimo de 25.000 pesos al mes. Para vender el POS, lo que hay que resolver con ellos es si ese mínimo se puede agrupar y si el alta de un NIT nuevo se puede hacer sin intervención manual.
- **Ir directo es viable para el NIT propio** (Laguna) como software propio. Para facturar a nombre de otras organizaciones hay una zona gris legal que necesita concepto de un tributarista antes de escribir código.

## Mensaje para LaFactura

Texto listo para enviar; ajusta el saludo y la firma.

> Hola, equipo de LaFactura:
>
> Hoy uso su servicio para la facturación electrónica de mi restaurante y estoy contento con el resultado. Estoy por empezar a vender mi sistema POS a otros restaurantes, y cada uno facturará con su propio NIT a través de mi plataforma, que ya está integrada con su API.
>
> Antes de salir a vender quiero entender cómo trabajaríamos juntos en ese esquema. Tengo estas preguntas:
>
> 1. **Mínimo mensual.** Hoy pago 30 pesos por factura con un mínimo de 25.000 pesos al mes. ¿Ese mínimo aplicaría a cada NIT nuevo, o puede haber un solo mínimo para mi cuenta que agrupe el consumo de todas las organizaciones?
> 2. **Precio de aliado.** ¿Tienen un esquema de aliado o revendedor con precio por volumen?
> 3. **Alta de un NIT nuevo.** ¿Puedo crear la organización y obtener sus credenciales por API, o cada alta pasa por ustedes de forma manual? ¿Cuánto tarda normalmente?
> 4. **Documentos y habilitación.** ¿Cuál es la lista exacta de documentos que debe entregar cada cliente? ¿Quién hace el registro y el set de pruebas ante la DIAN?
> 5. **Certificado digital.** ¿Lo gestionan ustedes o lo compra cada cliente? ¿Qué costo y vigencia tiene?
> 6. **Tipos de documento.** ¿La API soporta notas crédito, notas débito y documento equivalente electrónico POS, además de factura y documento soporte?
> 7. **Portabilidad.** ¿Puedo descargar los XML firmados de cada organización? Si un cliente cambia de proveedor, ¿conserva sus prefijos y su numeración?
>
> Con esas respuestas puedo definir el precio de mi producto y el proceso de alta de mis clientes. Si les queda más fácil, lo conversamos en una llamada.
>
> Gracias.

Las preguntas 1 y 3 son las que deciden si se puede sumar clientes sin costo ni trabajo extra por cada uno. Si la respuesta al mínimo agrupado es no, el peor caso son 25.000 pesos al mes por cliente.

## Los números

Con el precio actual, la conexión directa solo se paga con volumen alto. Todos los escenarios suponen que el mínimo aplica por NIT, que es el peor caso.

| Escenario | Costo mensual en LaFactura (COP) | Costo anual (COP) |
| --- | --- | --- |
| 1 organización con menos de 833 facturas al mes | 25.000 | 300.000 |
| 10 organizaciones pequeñas | 250.000 | 3.000.000 |
| 50 organizaciones pequeñas | 1.250.000 | 15.000.000 |
| 100 organizaciones con 3.000 facturas al mes cada una | 9.000.000 | 108.000.000 |

- **Punto de equilibrio del mínimo:** 25.000 / 30 ≈ 833 facturas al mes. Por debajo se paga el mínimo; por encima se paga por uso.
- **Lectura:** hasta unas decenas de organizaciones, LaFactura cuesta menos que construir y mantener la integración propia. El costo por cliente cabe en el precio de la suscripción del POS.
- **Señal para reabrir la decisión:** cuando la factura mensual de LaFactura se acerque al costo de un desarrollador dedicado al mantenimiento, o si no hay acuerdo sobre el alta de clientes.

## Marco legal

La DIAN admite tres modos de operación del software, y cuál aplica depende de quién firma y transmite. La norma base es la [Resolución DIAN 000165 de 2023](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0165_2023.htm), artículo 28.

| Modo (art. 28, numeral 2) | Quién firma y transmite | Para qué sirve aquí |
| --- | --- | --- |
| 2.1 Desarrollo informático propio o adquirido | El facturador, directamente | Laguna con su propio NIT; es también el modo en que opera LaFactura con sus clientes |
| 2.2 Servicio gratuito de la DIAN | El portal de la DIAN | No es integrable con un POS |
| 2.3 Proveedor tecnológico (PT) | El PT habilitado, por cuenta del facturador | La figura pensada para una plataforma que factura por terceros |

### Para el NIT propio

Software propio, sin ambigüedad. El [Concepto DIAN 013246 de 2025](https://normograma.dian.gov.co/dian/compilacion/docs/oficio_dian_13246_2025.htm) define software propio como el que "desarrolla directamente el facturador" y adquirido como el que "le compra a un tercero".

### Para otras organizaciones

Cada cliente es un facturador con su NIT. Hay dos caminos.

**Camino A: desarrollo informático adquirido.** Cada cliente registra el software ante la DIAN indicando el NIT del fabricante, pasa su propio set de pruebas y firma con su propio certificado. La resolución lo permite sin exigir que el fabricante sea PT (art. 28, numerales 2.1 y 3.1).

**Camino B: habilitarse como proveedor tecnológico (art. 55).** Requisitos principales:

- Sociedad constituida en Colombia o sucursal de sociedad extranjera.
- Patrimonio igual o superior a 20.000 UVT y propiedad, planta y equipo igual o superior a 10.000 UVT, ambos localizados en Colombia. Con la UVT de 2026 son aproximadamente 1.050 y 525 millones de pesos (cálculo propio; verificar el valor de la UVT).
- Certificación ISO 27001, o compromiso de aportarla dentro de los 18 meses siguientes a la habilitación.
- Estar habilitado como facturador electrónico, plan de contingencia, niveles de servicio, personal profesional y visita de verificación.
- Vigencia de 5 años; la renovación se pide con al menos 3 meses de anticipación.

### La zona gris del camino A

El artículo 55 dice que la generación y transmisión "será realizada directamente por el facturador electrónico", y el artículo 63 exige que un tercero que cumpla esas obligaciones esté "previamente autorizado y/o habilitado". Si la nube de la plataforma firma y transmite por los clientes, se parece a un PT sin habilitación.

Esto es una interpretación, no texto literal de la norma. Que LaFactura opere así muestra que el modelo se usa en el mercado, pero no prueba que la DIAN lo acepte para una plataforma en la nube. Necesita concepto de un tributarista o consulta escrita a la DIAN.

Una opción de diseño reduce el riesgo: firmar y transmitir desde la caja edge instalada en el local del cliente, con el certificado del cliente. Así quien opera el software es el facturador.

## Costos de la conexión directa

La DIAN no cobra, pero ir directo cambia un costo variable pequeño por costos fijos y riesgo propio. En la Resolución 165 y en las páginas de la DIAN consultadas no aparece ninguna tarifa por habilitación ni por validación.

| Rubro | Quién lo paga | Frecuencia | Nota |
| --- | --- | --- | --- |
| Certificado digital por NIT, de una entidad acreditada por ONAC | Cada organización | Cada 1 o 2 años | Sin precio vigente verificado; cotizar con Certicámara, GSE y Andes |
| Habilitación y set de pruebas por NIT | Tiempo de soporte propio | Una vez por cliente | En el camino A cada cliente la hace |
| Desarrollo inicial | Propio | Una vez | El rubro más grande; ver las fases |
| Mantenimiento normativo | Propio | Permanente | Cada anexo técnico nuevo obliga a actualizar con fecha límite |
| Operación y soporte | Propio | Permanente | Rechazos, caídas de la DIAN y contingencia dejan de ser problema del proveedor |
| Asesoría legal | Propio | Una vez | Concepto sobre el camino A antes de servir a terceros |

## Qué hay que construir

La integración directa son cinco piezas: generar el XML, firmarlo, transmitirlo, administrar la numeración y entregar el documento al cliente. Los detalles exactos de cada una están en los anexos técnicos de la DIAN, que no se leyeron para este documento; lo de esta sección es el mapa, no la especificación.

### Documentos y sus anexos

| Documento | Norma o anexo | Identificador | Habilitación |
| --- | --- | --- | --- |
| Factura electrónica de venta | Anexo técnico 1.9 (Res. 165 de 2023) | CUFE | Set de pruebas de factura |
| Nota crédito y nota débito | Mismo anexo 1.9 | CUDE | Dentro del mismo set de pruebas |
| Documento soporte y su nota de ajuste | [Resolución 000167 de 2021](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0167_2021.htm) y su anexo | CUDS | Habilitación aparte |
| Documento equivalente electrónico POS | Anexo técnico 1.0 (Res. 165 de 2023) | CUDE | Habilitación aparte; no admite el servicio gratuito |

El documento equivalente POS no estaba en la lista original, pero aplica a restaurantes. Confirmar con el contador si se emite factura electrónica para todas las ventas o si hace falta.

### Piezas técnicas

1. **Generación del XML.** Formato UBL 2.1 con las extensiones de la DIAN: datos del software, del fabricante, rango de numeración y código QR. La factura debe incluir el NIT del fabricante y el nombre del software (art. 11, numeral 18).
2. **Identificadores.** CUFE, CUDE y CUDS se calculan con SHA-384 sobre campos del documento. El CUFE usa la clave técnica que la DIAN genera al asociar la numeración (art. 28, numeral 3.5). También se calcula el código de seguridad del software.
3. **Firma digital.** XAdES según la política de firma de la DIAN, con el certificado del facturador (art. 11, numeral 14).
4. **Transmisión.** Servicio web SOAP 1.2 con WS-Security; el XML viaja comprimido en ZIP. Ambiente de habilitación en `vpfe-hab.dian.gov.co` y otro de producción. Métodos principales: `SendTestSetAsync` para el set de pruebas, `SendBillSync` para producción, `GetStatus` y `GetStatusZip` para consultar, `GetNumberingRange` para la numeración.
5. **Respuesta de la DIAN.** Guardar el documento de validación. Si la DIAN rechaza, se corrige, se firma de nuevo y se retransmite con el mismo número, sin nota crédito.
6. **Entrega al cliente.** Representación gráfica en PDF con QR y el contenedor electrónico firmado, enviados por correo.
7. **Numeración.** Solicitar rangos, asociar prefijos al software, vigilar agotamiento y vencimiento.
8. **Contingencia.** Qué se expide cuando la DIAN o el sistema propio no responden, y cómo se transmite después.
9. **Opcional: eventos RADIAN.** Acuse de recibo y aceptación de facturas de compra.

### Datos que hay que guardar por organización

- Certificado digital y su clave, cifrados, con fecha de vencimiento.
- Identificador del software y PIN registrados en la DIAN.
- Identificador del set de pruebas y estado de habilitación por tipo de documento.
- Prefijos, rangos de numeración, vigencia y clave técnica.
- XML firmado y respuesta de la DIAN de cada documento.

### Cómo encaja en el código actual

- El puerto `ElectronicInvoiceClient` (`backend/internal/domain/ports/electronic_invoice_client.go`) ya aísla al proveedor: un adaptador de DIAN directa implementaría el mismo contrato que hoy cumple el de LaFactura (`backend/internal/platform/httpclient/electronic_invoice.go`).
- El puerto solo cubre factura, documento soporte y consulta. Notas crédito y débito son trabajo nuevo con cualquier proveedor.
- Los prefijos y las credenciales ya son por organización.
- No se identificó una librería madura en Go para XAdES con la política de la DIAN. Las referencias abiertas están en PHP ([ubl21dian](https://packagist.org/packages/dcorreah/ubl21dian)) y en un cliente SOAP de ejemplo ([soap-dian](https://github.com/Stenfrank/soap-dian)). La firma y la canonicalización del XML son la parte con más riesgo técnico.

## Plan por fases

El plan arranca con el NIT propio, que es legalmente seguro, y deja lo multi-organización para cuando haya concepto jurídico. Solo se ejecuta si no hay acuerdo con LaFactura o si el volumen lo justifica.

### Fase 0: decisión (sin código)

- [ ] Enviar el mensaje a LaFactura y esperar respuesta sobre mínimo agrupado y alta por API.
- [ ] Revisar en el portal de la DIAN qué NIT figura como fabricante del software en el registro actual de Laguna.
- [ ] Pedir concepto a un tributarista sobre el camino A para una plataforma en la nube.
- [ ] Confirmar con el contador si se necesita documento equivalente POS.
- [ ] Cotizar certificados digitales con dos o tres entidades.

### Fase 1: Laguna en ambiente de habilitación

- [ ] Descargar los anexos técnicos vigentes y la caja de herramientas de la DIAN (ejemplos de XML).
- [ ] Comprar el certificado digital para el NIT de Laguna.
- [ ] Registrar un software nuevo como desarrollo propio; obtener identificador, PIN e identificador del set de pruebas.
- [ ] Construir generación de XML, identificadores, firma y cliente SOAP para factura, nota crédito y nota débito.
- [ ] Pasar el set de pruebas de factura. La cantidad de documentos exigida la indica el portal.
- [ ] Repetir para documento soporte y, si aplica, documento equivalente POS.

### Fase 2: producción para Laguna

- [ ] Solicitar numeración con un prefijo nuevo y asociarla al software; obtener la clave técnica.
- [ ] Construir PDF, contenedor electrónico y envío por correo.
- [ ] Definir e implementar la contingencia.
- [ ] Operar en paralelo con LaFactura en el prefijo nuevo antes de migrar todo. Fijar la fecha de inicio con cuidado: una vez seleccionada no se puede modificar (art. 28, numeral 3.6).
- [ ] Agregar alertas de vencimiento del certificado y de agotamiento de numeración.

### Fase 3: otras organizaciones

Solo si el concepto jurídico de la fase 0 lo respalda.

- [ ] Decidir dónde se firma y transmite: nube o caja edge del cliente.
- [ ] Guardar por organización los datos de la sección anterior.
- [ ] Construir el flujo guiado de alta: documentos, compra del certificado, registro del software, set de pruebas y numeración.
- [ ] Permitir elegir proveedor por organización, para que LaFactura y la conexión directa convivan durante la migración.

## Riesgos y puntos sin verificar

El riesgo mayor es legal, no técnico: facturar por terceros sin ser proveedor tecnológico.

**Riesgos**

- **Figura legal para terceros.** Si la DIAN entiende que la plataforma transmite por cuenta de los clientes, exigiría habilitación como PT (art. 63).
- **Firma XML en Go.** Sin librería madura, es el punto donde más se puede alargar el desarrollo.
- **Cambios normativos.** Los artículos de la Resolución 165 fueron compilados en la Resolución 227 de 2025; los anexos cambian de versión con fechas obligatorias.
- **Custodia de certificados.** Guardar las claves de firma de los clientes es una responsabilidad de seguridad que hoy no existe.
- **Fricción de alta.** Con cualquier camino, cada cliente debe conseguir certificado y habilitarse; eso frena la venta más que el precio por factura.

**No verificado para este documento**

- Texto del artículo 616-4 del Estatuto Tributario y del artículo 1.6.1.4.24 del Decreto 1625 de 2016.
- Texto de la Resolución 227 de 2025.
- Anexos técnicos y política de firma (PDF de la DIAN): estructura exacta del XML, fórmulas de los identificadores, cantidades del set de pruebas.
- Precios vigentes de certificados digitales.
- Valor exacto de la UVT de 2026 usado en la conversión a pesos.
- Si LaFactura figura o no en el listado de proveedores tecnológicos habilitados.
- Que no exista ninguna tarifa de la DIAN: no se encontró ninguna, pero tampoco una norma que declare gratuito el web service.

## Fuentes

- [Resolución DIAN 000165 de 2023](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0165_2023.htm): artículos 11, 19, 28, 36, 38, 49, 55 a 57 y 63.
- [Concepto DIAN 013246 de 2025](https://normograma.dian.gov.co/dian/compilacion/docs/oficio_dian_13246_2025.htm): software propio y adquirido.
- [DIAN: preguntas y respuestas de proveedores tecnológicos](https://micrositios.dian.gov.co/sistema-de-facturacion-electronica/preguntas-y-respuestas-de-proveedores-tecnologicos/).
- [Resolución DIAN 000167 de 2021](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0167_2021.htm): documento soporte (solo resultado de búsqueda, no leída completa).
- [DIAN: instructivo de registro y habilitación](https://micrositios.dian.gov.co/sistema-de-facturacion-electronica/instructivo-de-registro-y-habilitacion-en-factura-electronica-dian/) (solo resultado de búsqueda).
- [ubl21dian](https://packagist.org/packages/dcorreah/ubl21dian) y [soap-dian](https://github.com/Stenfrank/soap-dian): implementaciones abiertas de referencia (solo resultado de búsqueda).
- Precios y modalidad de LaFactura: informados por el autor.
