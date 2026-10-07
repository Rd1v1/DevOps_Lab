# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder
WORKDIR /src
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .
# Optional release metadata; tokens must never be embedded into public JS.
ARG BUILD_VARIANT=local
RUN --mount=type=secret,id=release \
    set -eu; export VITE_RELEASE=local; \
    if [ -f /run/secrets/release ]; then export VITE_RELEASE="$(cat /run/secrets/release)"; fi; \
    npm run build -- --outDir "/out/$BUILD_VARIANT" --emptyOutDir

FROM nginx:1.28-alpine AS runtime
ARG BUILD_VARIANT=local
RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /out/${BUILD_VARIANT}/ /usr/share/nginx/html
USER nginx
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --retries=5 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
CMD ["nginx", "-g", "daemon off;"]
