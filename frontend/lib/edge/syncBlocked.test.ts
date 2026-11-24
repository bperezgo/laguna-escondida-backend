import { test } from "node:test";
import assert from "node:assert/strict";
import { syncBlockedMessage } from "./syncBlocked.ts";

test("a replaced box tells an admin or manager", () => {
  assert.equal(
    syncBlockedMessage("replaced", true),
    "Esta caja fue reemplazada por otra. Ya no sincroniza con la nube."
  );
});

test("a suspended account tells an admin or manager", () => {
  assert.equal(
    syncBlockedMessage("suspended", true),
    "La cuenta está suspendida. Esta caja no sincroniza con la nube."
  );
});

test("nothing blocked shows nothing", () => {
  assert.equal(syncBlockedMessage(null, true), null);
  assert.equal(syncBlockedMessage(undefined, true), null);
});

test("a server role sees nothing", () => {
  assert.equal(syncBlockedMessage("replaced", false), null);
  assert.equal(syncBlockedMessage("suspended", false), null);
});
