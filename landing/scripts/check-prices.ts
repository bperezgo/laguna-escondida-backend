import { annualPrice, plans } from "../src/config/plans.ts";
import { comparePrices, parsePlanPrices, type PlanPrice } from "./compare-prices.ts";

function fail(message: string): never {
  console.error(`Price check failed: ${message}`);
  process.exit(1);
}

const base = process.env.PUBLIC_API_BASE_URL?.trim().replace(/\/+$/, "");
if (!base) fail("PUBLIC_API_BASE_URL is not set.");

const url = `${base}/api/plans`;
let cloud: PlanPrice[];
try {
  const response = await fetch(url, { headers: { Accept: "application/json" }, signal: AbortSignal.timeout(20_000) });
  if (!response.ok) throw new Error(`it answered ${response.status}`);
  cloud = parsePlanPrices(await response.json());
} catch (error) {
  fail(`could not read the cloud's prices from ${url}: ${error instanceof Error ? error.message : String(error)}. Is the cloud up? Rerun the release once it is.`);
}

const site = plans.map((plan) => ({ id: plan.id, monthly: plan.monthly, annual: annualPrice(plan) }));
const problems = comparePrices(site, cloud);
if (problems.length > 0) {
  fail(`src/config/plans.ts and ${url} differ:\n${problems.map((line) => `  - ${line}`).join("\n")}`);
}
console.log(`Prices match ${url}: ${site.map((plan) => plan.id).join(", ")}.`);
