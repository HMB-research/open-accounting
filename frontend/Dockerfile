# Build stage - use Bun for fast dependency installation and build
FROM oven/bun:1-alpine AS builder

WORKDIR /app

# Copy package files and lockfile
COPY package.json bun.lock ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy source code
COPY . .

# Build the application
RUN bun run build

# Production stage - use Node.js for runtime (adapter-node output)
FROM node:22-alpine

WORKDIR /app

# Copy built application and package files
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json /app/bun.lock ./

# Install production dependencies only with Bun for speed
COPY --from=oven/bun:1-alpine /usr/local/bin/bun /usr/local/bin/bun
RUN bun install --production --frozen-lockfile

# Use the built-in node user (already exists in node:alpine)
USER node

EXPOSE 3000

ENV NODE_ENV=production
ENV PORT=3000

CMD ["node", "build"]
