// PIN that gates irreversible order actions (remove a bill / pay a bill).
//
// Read as a static `process.env.NEXT_PUBLIC_*` reference so Next.js inlines it
// into the client bundle at build time.
//
// This is NOT a security control — the value ships in the JS. It only forces a
// deliberate step so a mis-tap on the touch floor screen can't remove or pay a
// bill by accident.
//
// When the env var is unset (e.g. the cloud deployment), the gate is DISABLED
// and actions run immediately as before. It's meant to be defined only on the
// edge deployment, where the extra confirmation is wanted.
export const ORDER_ACTION_PIN = process.env.NEXT_PUBLIC_ORDER_ACTION_PIN ?? "";

export const isOrderActionPinRequired = ORDER_ACTION_PIN.length > 0;
