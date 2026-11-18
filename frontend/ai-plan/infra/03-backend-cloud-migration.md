# Plan 3: Backend Cloud Migration

## Context

The cloud backend currently runs on Digital Ocean (simplest tier). The user reports
that SSE behaves badly on this setup. The proposed move is **ECS Fargate behind an
ALB**, on the assumption that we need a customizable network for long-lived SSE
connections.

Before committing to ECS, this plan re-frames the problem with two facts that change
the requirements:

1. **After plan 1, the cloud is no longer the source of truth during service.** The
   KDS/register/manager devices on the staff VLAN talk to the local server. The
   cloud's job becomes: receive sync events, host off-site dashboards, store
   backups, and (eventually) aggregate multi-location data.
2. **SSE on the LAN is served by the local Go backend**, not the cloud. The Ctrl+P /
   kitchen-display latency problem is entirely a LAN problem after plan 1.

That means the cloud's only remaining SSE need is for **off-site dashboards** (an
owner watching live orders from home). That is a much smaller and more relaxed
requirement than "the kitchen depends on this stream."

This reordering changes the answer.

## Diagnose before migrating

Verify the DO SSE problem is not a 60-second idle-timeout misconfiguration. DO App
Platform's default load balancer idle timeout is short; long-lived SSE drops at
~60 s if the backend doesn't send a heartbeat. Two quick experiments:

1. Add a `:heartbeat\n\n` SSE comment every 15 s from the Go backend. Re-test SSE on
   DO. If it now stays open, the migration was never strictly required.
2. Confirm the DO LB / proxy / CDN in front of the app isn't buffering SSE. Set
   `X-Accel-Buffering: no` (already present in
   `app/api/sse/commands/[area]/route.ts:69`) and verify it propagates.

If both are already done and SSE still misbehaves on DO, then yes — migrate. But
even then, ECS Fargate is not the only option.

## Target options, ranked for this scale

| Option | Monthly cost (est) | SSE support | Ops overhead | Fit |
|--------|-------------------|-------------|--------------|-----|
| **Fly.io** | $5–15 | Native, no LB idle-timeout footgun | Low (single CLI deploy) | Best fit for a single-restaurant cloud sink |
| **Render** | $7–25 | Native | Low | Same league as Fly |
| **Railway** | $5–20 | Native | Low | Same league |
| **DO Droplet + Caddy/nginx + Postgres** | $12–24 | Fine if you control the proxy | Medium (you manage the VM) | Good "no platform magic" choice |
| **AWS ECS Fargate + ALB** | $40–80 | Yes, ALB idle-timeout configurable to 4000 s | High (VPC, ALB, ECR, IAM, secrets) | Right answer only if AWS is strategic |
| **Cloudflare Workers / Containers** | Variable | Workers OK for short SSE; Containers OK for long | Low | Possible but newer; skip for now |

**Recommendation: Fly.io** for the cloud-sink role. Reasoning:

- After plan 1, the cloud workload is small: ingest sync events, serve dashboards
  for a few off-site users. ECS Fargate + ALB starts at ~$40/mo just for the ALB
  before any traffic. Fly.io starts at ~$2 for the same compute and includes a
  global Anycast frontend that handles SSE without the idle-timeout footgun.
- Fly is a single binary deploy: `flyctl deploy` against the existing Dockerfile.
  The current Dockerfile (multi-stage, alpine runtime) drops in with no changes.
- Managed Postgres on Fly is $5–15/mo for this size. Or keep DO Managed Postgres for
  continuity and point Fly at it.

ECS Fargate stays on the table for **two reasons only**:

1. You already use AWS strategically for other workloads and want one cloud.
2. You expect multi-location growth where ECS's primitives (service discovery, IAM
   integration, eventually EKS) start paying for themselves.

If neither is true, picking Fargate is over-investment.

## If you do migrate to ECS Fargate

This section is for the case where AWS is the chosen target despite the above.

### Network topology

```
                     ┌──────────────────────┐
                     │   Route 53 (DNS)     │
                     └──────────┬───────────┘
                                │
                     ┌──────────▼───────────┐
                     │   ACM (TLS cert)     │
                     └──────────┬───────────┘
                                │
┌─────────────────────────  VPC  ──────────────────────────────────────┐
│                                                                      │
│  Public subnets (2 AZs)         Private subnets (2 AZs)              │
│  ┌──────────────┐               ┌──────────────────────────────────┐ │
│  │  ALB (HTTPS) │  ───────────► │  ECS Service (Fargate)           │ │
│  │  idle=3600s  │               │  - 1 task, 0.25 vCPU / 0.5 GB    │ │
│  │  443 → 8080  │               │  - autoscale 1–3 on CPU > 70%   │ │
│  └──────────────┘               └──────────────┬───────────────────┘ │
│                                                │                     │
│                                  ┌─────────────▼──────────────────┐  │
│                                  │ RDS Postgres (db.t4g.micro)    │  │
│                                  │ Multi-AZ optional              │  │
│                                  └────────────────────────────────┘  │
│                                                                      │
│  NAT Gateway (for ECS task outbound: pull images, hit APIs)          │
└──────────────────────────────────────────────────────────────────────┘
```

### Key settings for SSE

- **ALB target group attributes**: `deregistration_delay.timeout_seconds = 30`,
  `load_balancing.algorithm.type = least_outstanding_requests`.
- **ALB listener idle timeout**: 3600 s (max 4000). Default is 60 s — would kill
  SSE immediately. **This is the single most important setting.**
- ECS task: send SSE heartbeats every 15 s anyway, for client-side keep-alive too.
- Health check on `/api/health`, interval 30 s, threshold 2.
- ECS minimum healthy %: 100. Maximum percent: 200 (rolling deploy).
- Use **App Runner only if** you accept the 120-second per-request timeout — which
  **breaks SSE**. Do not use App Runner for this workload.

### Cost-conscious settings

- Fargate Spot for non-critical tasks (after autoscale kicks in). Keep at least one
  on-demand baseline task.
- ARM (`arm64`) Fargate is ~20% cheaper than x86. Add `linux/arm64` to the
  Dockerfile build target.
- Single NAT Gateway is ~$32/mo. If only one AZ in use during quiet months, that's
  the single biggest line item — consider replacing with VPC interface endpoints
  for ECR, Secrets Manager, CloudWatch to drop egress traffic, or use a NAT
  instance (~$5/mo) for low traffic.

### Secrets

- Move `.env` to SSM Parameter Store (free for standard parameters) or Secrets
  Manager ($0.40/secret/mo).
- ECS task definition references parameters; tasks load them at boot via the IAM
  task execution role.

### CI/CD

- Push image to ECR on merge to main.
- ECS service update via `aws ecs update-service --force-new-deployment` from
  GitHub Actions.
- Tag images with git sha for rollback; don't rely on `latest`.

## What changes after plan 1 lands

Re-evaluate this whole plan once the local server is the source of truth. The cloud
backend is then much simpler:

- Sync ingest endpoint (idempotent writes from local servers)
- Read-only API for dashboards
- Auth issuer (JWT) and user/permission CRUD
- Cron jobs: nightly backup verification, sync-lag alerting

Most expensive pieces of a "real" cloud backend (high concurrency, low latency,
critical path) **go away** once the LAN owns the critical path. That is the
strongest argument for downsizing the cloud target.

## Addressing the user's explicit SSE question

> If we move to a desktop app, will we have access to SSE events easily?

Yes, trivially. SSE is an HTTP-level pattern. Inside the desktop app's WebView2 (see
plan 2), the browser `EventSource` API works exactly as it does today on
`app/api/sse/commands/[area]/route.ts`. From a native Go bridge, `http.Get` + a
streaming reader works too. The desktop-app question and the cloud-SSE question are
independent.

The deeper point: **after plan 1, the SSE that matters is LAN-local**, served by the
Go backend on the register PC. The cloud-side SSE only matters for off-site users,
and that path tolerates re-connects and small gaps just fine.

## Migration sequence (if migrating)

1. **Diagnose first.** Run the two SSE experiments above against DO. Decide.
2. **Decide target.** Fly.io for cost/simplicity (recommended), ECS Fargate for AWS
   strategy.
3. **Provision** the new environment in a separate database, with no production
   traffic.
4. **Dual-write window** (1–2 weeks): backend writes to both old and new DB. Read
   traffic still on DO.
5. **Switch reads** to new env in low-traffic window. Monitor for 48 h.
6. **Decommission DO** once new env has been stable for 7 days.
7. **Document the rollback path** explicitly: keep DO Postgres snapshot for 30 days
   after cutover.

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Underestimating ALB idle-timeout impact (SSE drops mid-stream) | Set to 3600 s explicitly; integration-test with a 1-hour SSE client in CI |
| NAT Gateway cost surprise | Pre-compute monthly bill with `aws-pricing-calculator`; consider NAT instance or VPC endpoints |
| Migration during service hours | Cut over in early morning, with feature flag pointing frontend to old API as fallback |
| Postgres extension/version drift between DO and target | Snapshot, pg_dump, restore in new env, run schema diff before cutover |
| Cloud-side SSE goes down — does the restaurant care? | After plan 1: no. Document this so on-call doesn't escalate cloud SSE outages during dinner service. |

## Open questions for the next session

- Has the SSE-heartbeat experiment been run against DO? If yes, what was the result?
- Is AWS chosen for strategic reasons, or is this a pure cost/quality decision?
- After plan 1, who actually consumes cloud SSE? If the answer is "nobody yet,"
  consider removing cloud SSE entirely and serving off-site dashboards via polling.
