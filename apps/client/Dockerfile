# Build stage
FROM oven/bun:1-alpine AS builder

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .
RUN bun run build

# Production stage
FROM oven/bun:1-alpine AS runner

WORKDIR /app

COPY --from=builder /app/dist /dist
COPY scripts/serveCloudRun.ts ./scripts/serveCloudRun.ts

EXPOSE 5173

CMD ["bun", "run", "scripts/serveCloudRun.ts"]
