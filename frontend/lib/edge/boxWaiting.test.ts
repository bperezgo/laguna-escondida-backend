import { test } from "node:test";
import assert from "node:assert/strict";
import { BOX_WAITING_TITLE, boxWaitingGuidance } from "./boxWaiting.ts";

test("the waiting screen names what it waits for", () => {
  assert.equal(BOX_WAITING_TITLE, "Esperando a que lleguen los usuarios desde la nube…");
});

test("waiting says users arrive soon and where to check in the cloud", () => {
  assert.match(boxWaitingGuidance("waiting"), /uno o dos minutos/);
  assert.match(boxWaitingGuidance(undefined), /uno o dos minutos/);
});

test("offline points at the network", () => {
  assert.match(boxWaitingGuidance("offline"), /conexión a internet/);
});

test("failed says the box could not receive the data and names the logs, not the password or the network", () => {
  const message = boxWaitingGuidance("failed");
  assert.match(message, /no pudo recibir los datos/);
  assert.ok(message.includes("C:\\kuentapos\\logs"));
  assert.doesNotMatch(message, /contraseña|internet/);
});

test("replaced and suspended reuse the POS messages", () => {
  assert.match(boxWaitingGuidance("replaced"), /reemplazada/);
  assert.match(boxWaitingGuidance("suspended"), /suspendida/);
});
