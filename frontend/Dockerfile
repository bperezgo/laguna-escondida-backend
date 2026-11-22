# Cloud image of the operations UI (Next.js standalone), run by aws-infra as the
# `laguna-web` Fargate service. The edge box does NOT use this: it runs the Windows
# bundle from release.yml (frontend-windows).

# Build stage
FROM node:22-alpine AS builder
WORKDIR /app

# pnpm at the exact version package.json pins (corepack reads `packageManager`).
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

# Only NEXT_PUBLIC_ORDER_ACTION_PIN is inlined into the browser bundle (fixed for now,
# same value as the Windows bundle). NEXT_PUBLIC_API_URL is read by the Next *server*
# at runtime (the browser never calls the backend), so it is NOT baked in: the value
# below only lets `next build` import the config module, and the task overrides it.
ARG NEXT_PUBLIC_ORDER_ACTION_PIN=1234
ENV NEXT_PUBLIC_ORDER_ACTION_PIN=${NEXT_PUBLIC_ORDER_ACTION_PIN} \
    NEXT_PUBLIC_API_URL=http://127.0.0.1:8080/api \
    NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

# Runtime stage: standalone server + static assets, nothing else.
FROM node:22-alpine
WORKDIR /app

ARG VERSION=dev
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0 \
    APP_VERSION=${VERSION}

# server.js sits at the root of .next/standalone (next.config.js pins the tracing root).
COPY --from=builder --chown=node:node /app/.next/standalone ./
COPY --from=builder --chown=node:node /app/.next/static ./.next/static
COPY --from=builder --chown=node:node /app/public ./public

USER node
EXPOSE 3000
CMD ["node", "server.js"]
