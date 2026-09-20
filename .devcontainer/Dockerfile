# Build FROM the maintained Dev Containers Ubuntu base and add only what the egress
# firewall needs plus a few ownership fixes. The toolchain (Go, Node, Claude Code,
# Docker CLI) is layered on top by "features" in devcontainer.json, so this image
# stays small and the environment stays standard.
FROM mcr.microsoft.com/devcontainers/base:ubuntu

# Packages the firewall needs:
#   iptables, ipset -> the rule engine + efficient IP-set matching
#   dnsutils (dig)  -> resolve allowlisted domains to their current IPs
#   curl, jq        -> fetch + parse GitHub's published server IP ranges
#   iproute2 (ip)   -> discover the gateway and the container's attached subnets
#                      (how the box reaches its own postgres/minio siblings)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        iptables ipset dnsutils curl jq iproute2 \
    && rm -rf /var/lib/apt/lists/*

# Bake the firewall script into the image under a PROJECT-SPECIFIC name.
#
# Not /usr/local/bin/init-firewall.sh: that is the well-known path used by Anthropic's
# reference devcontainer, and "features" install AFTER this Dockerfile runs — so a
# feature shipping its own init-firewall.sh silently overwrites ours, and
# postStartCommand then runs a script we never wrote. (Symptom: startup logs printing
# "Resolving <domain>..." / "ERROR: Failed to resolve ..." instead of our "[firewall] ..."
# lines, dying on a single unresolvable domain.) A distinct name makes the collision
# impossible in either direction.
COPY init-firewall.sh /usr/local/bin/laguna-firewall.sh
RUN chmod +x /usr/local/bin/laguna-firewall.sh

# Create every path a named volume mounts over, owned by vscode. Docker seeds a fresh
# named volume from whatever is at that path in the image — and when the path does NOT
# exist, it creates the mountpoint as root:root. Symptoms, one per line below:
#   ~/.claude        -> /login "succeeds", next command says "Not logged in" forever
#   go/pkg/mod       -> `go mod download` dies with permission denied
#   .cache/go-build  -> every build recompiles the world, or fails outright
#   go/bin           -> `go install` of the project toolchain fails
RUN mkdir -p /home/vscode/.claude /home/vscode/go/pkg/mod /home/vscode/go/bin /home/vscode/.cache/go-build \
    && chown -R vscode:vscode /home/vscode/.claude /home/vscode/go /home/vscode/.cache

# Let the non-root "vscode" user run as root ONLY these commands, without a password:
#   laguna-firewall.sh  -> so postStartCommand can apply the firewall
#   docker-init.sh      -> the docker-outside-of-docker feature's socat proxy; a
#                          manual restart is the documented fix when `docker ps` says
#                          permission denied inside the box (README troubleshooting).
#                          No extra privilege in practice: the mounted socket already
#                          grants host-daemon access to whoever can use it.
RUN printf '%s\n' \
      'vscode ALL=(root) NOPASSWD: /usr/local/bin/laguna-firewall.sh' \
      'vscode ALL=(root) NOPASSWD: /usr/local/share/docker-init.sh' \
      > /etc/sudoers.d/laguna-devbox \
    && chmod 0440 /etc/sudoers.d/laguna-devbox

# Revoke the base image's blanket sudo. mcr.microsoft.com/devcontainers/base ships
# /etc/sudoers.d/vscode granting `vscode ALL=(ALL) NOPASSWD: ALL` — which makes the
# narrow rules above decorative: anything running as vscode, including Claude in
# --dangerously-skip-permissions mode, could simply `sudo iptables -F` and undo the
# egress firewall that is the whole point of this box. Remove it, then ASSERT the end
# state in both directions: the firewall script is still reachable, and no rule
# surviving anywhere under /etc/sudoers.d grants blanket root. If a future base image
# renames that file, this fails the build loudly instead of quietly reopening the hole.
RUN rm -f /etc/sudoers.d/vscode \
    && sudo -l -U vscode > /tmp/sudo-l 2>&1 \
    && { grep -q 'laguna-firewall.sh' /tmp/sudo-l \
         || { echo "BUILD ERROR: vscode can no longer sudo the firewall script" >&2; exit 1; }; } \
    && { ! grep -qE 'NOPASSWD:[[:space:]]*ALL' /tmp/sudo-l \
         || { echo "BUILD ERROR: vscode still has blanket sudo — a broad rule survives under /etc/sudoers.d" >&2; exit 1; }; } \
    && rm -f /tmp/sudo-l
