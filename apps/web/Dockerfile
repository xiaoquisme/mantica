# --- Dependencies stage ---
FROM node:22-alpine AS deps

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
RUN npm config set registry https://registry.npmmirror.com

WORKDIR /app

# Copy workspace root files
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY packages/ ./packages/

# Copy only web app package.json for dependency resolution
COPY apps/web/package.json ./apps/web/

# Install all dependencies (including workspace packages)
RUN pnpm install --frozen-lockfile

# --- Build stage ---
FROM node:22-alpine AS builder

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate
RUN npm config set registry https://registry.npmmirror.com

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/packages ./packages
COPY --from=deps /app/apps/web/node_modules ./apps/web/node_modules
COPY package.json pnpm-workspace.yaml ./
COPY apps/web/ ./apps/web/

# Build-time env: point API rewrites to the Go backend container
# NEXT_PUBLIC_API_URL empty = use relative URLs (nginx proxy handles routing)
ARG NEXT_PUBLIC_API_URL=""
ARG NEXT_PUBLIC_WS_URL=""
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL}
ENV REMOTE_API_URL=http://server:8080

RUN cd apps/web && pnpm build

# --- Runtime stage ---
FROM node:22-alpine AS runner

RUN apk add --no-cache tini

WORKDIR /app

ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# Copy standalone output
COPY --from=builder /app/apps/web/.next/standalone ./
COPY --from=builder /app/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder /app/apps/web/public ./apps/web/public

EXPOSE 3000

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["node", "apps/web/server.js"]
