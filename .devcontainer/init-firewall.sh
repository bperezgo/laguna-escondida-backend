#!/bin/bash
# Egress firewall for the dev box.
#
# Purpose: when Claude Code runs with --dangerously-skip-permissions (zero prompts),
# a malicious or hijacked command must still be UNABLE to reach arbitrary hosts.
# We default OUTBOUND traffic to DROP and allow only a curated set of destinations
# (Anthropic, the Go/npm registries, GitHub, and this project's own siblings).
#
# Runs as root via sudo on EVERY container start (see postStartCommand in
# devcontainer.json) because iptables rules live in the network namespace and are
# wiped on restart.
set -euo pipefail

# ---------------------------------------------------------------------------
# Preflight. Name the cause instead of dying on a bare "iptables: Permission
# denied" buried in the startup log. Both of these fail *before* any rule is
# applied, which is exactly how the box ends up with wide-open egress.
# ---------------------------------------------------------------------------
if ! iptables -L -n >/dev/null 2>&1; then
  echo "[firewall] ERROR: iptables unusable. Check cap_add NET_ADMIN and NET_RAW are in" >&2
  echo "[firewall]        docker-compose.devcontainer.yml, and that the container was" >&2
  echo "[firewall]        RECREATED (Dev Containers: Rebuild Container) after they were" >&2
  echo "[firewall]        added — a plain restart keeps the old, capability-less container." >&2
  exit 1
fi
if ! ipset list >/dev/null 2>&1; then
  echo "[firewall] ERROR: ipset unusable — the host kernel can't load the ip_set modules." >&2
  exit 1
fi

# Fail CLOSED. Everything below runs while egress is still open (DNS + the GitHub
# ranges fetch need it), so an abort partway through would leave the container
# unrestricted — the failure mode that makes the leak check print LEAK. This trap
# turns that into "no network" instead of "no firewall".
firewall_ok=0
on_exit() {
  if [ "${firewall_ok}" -eq 1 ]; then
    return 0
  fi
  echo "[firewall] ERROR: setup did not complete — failing closed (new outbound dropped)." >&2
  echo "[firewall]        Fix the cause above, then: sudo /usr/local/bin/laguna-firewall.sh" >&2
  # Loopback + already-established connections stay up so the VS Code session and
  # your terminal survive; nothing new gets out.
  iptables -A OUTPUT -o lo -j ACCEPT 2>/dev/null || true
  iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || true
  iptables -P OUTPUT DROP 2>/dev/null || true
  return 0
}
trap on_exit EXIT

echo "[firewall] resetting existing rules ..."
iptables -F
iptables -X 2>/dev/null || true
ipset destroy allowed-domains 2>/dev/null || true
ipset create allowed-domains hash:net

# ---------------------------------------------------------------------------
# Allow-list. Hostnames are resolved NOW and their IPs added to the set; the rules
# below match on IP, so a CDN that rotates addresses mid-session needs a re-run.
# ---------------------------------------------------------------------------
ALLOWED_DOMAINS=(
  # --- Claude Code / Anthropic (required for Claude to function at all) ---
  "api.anthropic.com"
  "console.anthropic.com"
  "statsig.anthropic.com"
  "sentry.io"
  # claude.ai proxies the MCP servers configured for this project (Linear, Notion,
  # Docs). Drop it if you run the box without them.
  "claude.ai"
  # --- Go modules + checksum DB (the proxy serves module zips from Google storage) ---
  "proxy.golang.org"
  "sum.golang.org"
  "storage.googleapis.com"
  "go.dev"
  "golang.org"
  # govulncheck's vulnerability database — a separate host from go.dev, and the
  # reason a pre-push vuln scan times out if you forget it.
  "vuln.go.dev"
  # --- npm registry: Claude Code itself ships as an npm package ---
  "registry.npmjs.org"
)

for domain in "${ALLOWED_DOMAINS[@]}"; do
  echo "[firewall] resolving ${domain} ..."
  for ip in $(dig +short A "${domain}" | grep -E '^[0-9.]+$' || true); do
    ipset add allowed-domains "${ip}" 2>/dev/null || true
  done
done

# GitHub publishes its server IP ranges via api.github.com/meta. Allowing them keeps
# `go get`, git, and the golangci-lint / mockery / gotestsum / lefthook / migrate
# installers working (all hosted on github.com / *.githubusercontent.com).
echo "[firewall] adding GitHub IP ranges ..."
gh_meta="$(curl -fsSL --max-time 10 https://api.github.com/meta || echo '{}')"
for cidr in $(echo "${gh_meta}" | jq -r '((.web // []) + (.api // []) + (.git // []))[]' 2>/dev/null | sort -u); do
  ipset add allowed-domains "${cidr}" 2>/dev/null || true
done

# Directly-attached networks: the compose bridge this box shares with its own
# postgres/minio siblings. Without this the firewall would cut the box off from its
# own database — the traffic leaves via eth0, it is not loopback.
echo "[firewall] adding attached container subnets ..."
for cidr in $(ip -4 route show scope link | awk '{print $1}' | grep -E '^[0-9.]+/[0-9]+$' || true); do
  ipset add allowed-domains "${cidr}" 2>/dev/null || true
done

# The host machine: where Docker-outside-of-Docker publishes testcontainers' ports
# (TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal) and where a host-side stack
# would live. Both names below point at the host.
host_ip="$(getent hosts host.docker.internal | awk '{print $1}' | head -n1 || true)"
[ -n "${host_ip}" ] && ipset add allowed-domains "${host_ip}" 2>/dev/null || true
gateway="$(ip route | awk '/^default/ {print $3; exit}' || true)"
[ -n "${gateway}" ] && ipset add allowed-domains "${gateway}" 2>/dev/null || true

# ---------------------------------------------------------------------------
# Lock it down: default-DROP outbound; allow only the essentials + the allow-set.
# INPUT is left permissive so VS Code's tooling and port-forwarding keep working —
# egress is the exfiltration boundary that actually matters for this threat model.
# ---------------------------------------------------------------------------
echo "[firewall] applying default-deny egress ..."
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

iptables -A OUTPUT -o lo -j ACCEPT                                  # loopback
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT   # replies to us
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT                      # DNS
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT                      # DNS over TCP
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT # the allow-set

# IPv6 is a separate rule table and the allow-set above is IPv4-only: if the container
# ever has v6 connectivity, every rule above is bypassed by simply preferring AAAA.
# Nothing here needs v6, so drop it outright. Guarded so a kernel without the v6
# filter table warns instead of aborting — an abort here would trip the fail-closed
# trap and take the (working) IPv4 rules down with it.
if command -v ip6tables >/dev/null 2>&1 && ip6tables -L -n >/dev/null 2>&1; then
  ip6tables -F 2>/dev/null || true
  ip6tables -A OUTPUT -o lo -j ACCEPT 2>/dev/null || true
  ip6tables -P OUTPUT DROP 2>/dev/null || true
else
  echo "[firewall] WARNING: ip6tables unavailable — IPv6 egress is NOT filtered." >&2
fi

# ---------------------------------------------------------------------------
# Verify: a non-allowlisted host must be BLOCKED; Anthropic must be REACHABLE.
# (curl without -f: we test whether the connection succeeds, not the HTTP status.)
# ---------------------------------------------------------------------------
if curl -sS --max-time 5 -o /dev/null https://example.com 2>/dev/null; then
  echo "[firewall] ERROR: example.com is reachable — egress is NOT restricted!" >&2
  exit 1
fi
if ! curl -sS --max-time 5 -o /dev/null https://api.anthropic.com 2>/dev/null; then
  echo "[firewall] WARNING: api.anthropic.com unreachable — allowlist may be too tight." >&2
fi

firewall_ok=1
echo "[firewall] active — egress restricted to the allow-set."
