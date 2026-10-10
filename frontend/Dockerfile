# syntax=docker/dockerfile:1

# ---- build ----
FROM node:24-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY . .
# VITE_* values are compiled into the public bundle; never pass secrets here.
ARG VITE_APP_NAME=Boilerplate
ARG VITE_API_BASE=/api/v1
ENV VITE_APP_NAME=$VITE_APP_NAME VITE_API_BASE=$VITE_API_BASE
RUN npm run build

# ---- runtime ----
# nginx serves the SPA and reverse-proxies /api to the Go service, so both
# share one origin (see frontend/deploy/nginx.conf). Runs unprivileged on port 8080.
FROM nginxinc/nginx-unprivileged:1.30-alpine
COPY deploy/nginx.conf /etc/nginx/templates/default.conf.template
COPY deploy/security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY --from=build /app/dist /usr/share/nginx/html
# Upstream for /api; overridable at runtime (envsubst on the template).
ENV API_UPSTREAM=http://api:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/nginx-health || exit 1
