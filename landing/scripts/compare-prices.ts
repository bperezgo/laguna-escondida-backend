export interface PlanPrice {
  id: string;
  monthly: number;
  annual: number;
}

/** Reads the body of `GET /api/plans` (contract: backend/docs/api/plans.md). */
export function parsePlanPrices(body: unknown): PlanPrice[] {
  const list = (body as { plans?: unknown } | null)?.plans;
  if (!Array.isArray(list)) throw new Error("the answer has no `plans` list");
  return list.map((entry: unknown) => {
    const { id, monthly, annual } = (entry ?? {}) as Record<string, unknown>;
    if (typeof id !== "string" || typeof monthly !== "number" || typeof annual !== "number") {
      throw new Error(`a plan in the answer is malformed: ${JSON.stringify(entry)}`);
    }
    return { id, monthly, annual };
  });
}

/** One line per difference between the site's prices and the cloud's; empty when they agree. */
export function comparePrices(site: PlanPrice[], cloud: PlanPrice[]): string[] {
  const problems: string[] = [];
  const cloudById = new Map(cloud.map((plan) => [plan.id, plan]));

  for (const plan of site) {
    const other = cloudById.get(plan.id);
    if (!other) {
      problems.push(`plan "${plan.id}" is on the site but the cloud does not return it`);
      continue;
    }
    for (const period of ["monthly", "annual"] as const) {
      if (plan[period] !== other[period]) {
        problems.push(`plan "${plan.id}" ${period}: the site says ${plan[period]}, the cloud says ${other[period]}`);
      }
    }
  }

  const siteIds = new Set(site.map((plan) => plan.id));
  for (const plan of cloud) {
    if (!siteIds.has(plan.id)) problems.push(`plan "${plan.id}" is returned by the cloud but is not on the site`);
  }
  return problems;
}
