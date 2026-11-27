import { test } from "node:test";
import assert from "node:assert/strict";
import { encryptForWompi, TokenizeError, tokenizeCard } from "./wompiToken.ts";

const card = { number: "4242 4242 4242 4242", expiry: "12/29", cvc: "123", holder: " Rosa Pérez " };
const target = { publicKey: "pub_test_abc", apiUrl: "https://sandbox.wompi.co/v1/" };

async function keyPair() {
  const pair = await crypto.subtle.generateKey(
    { name: "RSA-OAEP", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
    true,
    ["encrypt", "decrypt"]
  );
  const spki = Buffer.from(await crypto.subtle.exportKey("spki", pair.publicKey)).toString("base64");
  const pem = `-----BEGIN PUBLIC KEY-----\n${spki.match(/.{1,64}/g)?.join("\n")}\n-----END PUBLIC KEY-----`;
  return { privateKey: pair.privateKey, pem, spki };
}

const fromBase64Url = (part: string) => Buffer.from(part, "base64url");

async function decrypt(jwe: string, privateKey: CryptoKey): Promise<string> {
  const [header, wrappedKey, iv, ciphertext, tag] = jwe.split(".");
  assert.deepEqual(JSON.parse(fromBase64Url(header).toString()), { alg: "RSA-OAEP-256", enc: "A256GCM" });
  const contentKey = await crypto.subtle.decrypt({ name: "RSA-OAEP" }, privateKey, fromBase64Url(wrappedKey));
  const aes = await crypto.subtle.importKey("raw", contentKey, "AES-GCM", false, ["decrypt"]);
  const plain = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: fromBase64Url(iv), additionalData: new TextEncoder().encode(header), tagLength: 128 },
    aes,
    Buffer.concat([fromBase64Url(ciphertext), fromBase64Url(tag)])
  );
  return new TextDecoder().decode(plain);
}

test("the payload decrypts to the five card fields, with a PEM or a bare key", async () => {
  const { privateKey, pem, spki } = await keyPair();
  for (const key of [pem, spki]) {
    const jwe = await encryptForWompi(key, JSON.stringify({ a: 1 }));
    assert.equal(jwe.split(".").length, 5);
    assert.equal(await decrypt(jwe, privateKey), '{"a":1}');
  }
});

test("a card goes to Wompi only, encrypted, and comes back as a token", async () => {
  const { privateKey, pem } = await keyPair();
  const calls: { url: string; init?: RequestInit }[] = [];
  const fakeFetch = (async (url: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(url), init });
    if (String(url).endsWith("/tokens/keys/tokenization")) return Response.json({ data: { publicKey: pem } });
    return Response.json({ status: "CREATED", data: { id: "tok_test_1_abc", brand: "VISA", last_four: "4242" } }, { status: 201 });
  }) as typeof fetch;

  const token = await tokenizeCard(target, card, fakeFetch);

  assert.deepEqual(token, { token: "tok_test_1_abc", brand: "visa", lastFour: "4242" });
  assert.deepEqual(calls.map((c) => c.url), ["https://sandbox.wompi.co/v1/tokens/keys/tokenization", "https://sandbox.wompi.co/v1/tokens/cards"]);
  for (const call of calls) {
    assert.ok(call.url.startsWith("https://sandbox.wompi.co/"), "the only address called is Wompi's");
    assert.equal((call.init?.headers as Record<string, string>).Authorization, "Bearer pub_test_abc");
  }
  const sent = String(calls[1].init?.body);
  assert.ok(!sent.includes("4242424242424242") && !sent.includes("123\""), "the card is never sent in the clear");
  const fields = JSON.parse(await decrypt(JSON.parse(sent).payload, privateKey));
  assert.deepEqual(fields, { number: "4242424242424242", cvc: "123", exp_month: "12", exp_year: "29", card_holder: "Rosa Pérez" });
});

test("a card Wompi refuses and a Wompi that does not answer are told apart", async () => {
  const { pem } = await keyPair();
  const answering = (status: number) =>
    (async (url: string | URL | Request) =>
      String(url).endsWith("/tokenization")
        ? Response.json({ data: { publicKey: pem } })
        : Response.json({ error: { type: "INPUT_VALIDATION_ERROR" } }, { status })) as typeof fetch;

  await assert.rejects(tokenizeCard(target, card, answering(422)), (e) => e instanceof TokenizeError && e.kind === "refused");
  await assert.rejects(tokenizeCard(target, card, answering(503)), (e) => e instanceof TokenizeError && e.kind === "unavailable");
  const down = (async () => {
    throw new TypeError("fetch failed");
  }) as typeof fetch;
  await assert.rejects(tokenizeCard(target, card, down), (e) => e instanceof TokenizeError && e.kind === "unavailable");
  const noKey = (async () => Response.json({ data: {} })) as typeof fetch;
  await assert.rejects(tokenizeCard(target, card, noKey), (e) => e instanceof TokenizeError && e.kind === "unavailable");
});
