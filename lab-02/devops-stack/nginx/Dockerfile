# syntax=docker/dockerfile:1
FROM alpine:3.22 AS openssl-builder
RUN apk add --no-cache openssl
COPY --chmod=755 generate-cert.sh /work/generate-cert.sh
WORKDIR /work
CMD ["/work/generate-cert.sh"]

FROM openssl-builder AS cert-builder
# Lab-only self-signed key is deliberately copied into the final image.
# A real private TLS key must be mounted at runtime instead.
RUN --mount=type=secret,id=cert_key,required=true \
    --mount=type=secret,id=cert_crt,required=true \
    mkdir -p /certs && cp /run/secrets/cert_key /certs/server.key \
    && openssl x509 -in /run/secrets/cert_crt -out /certs/server.crt \
    && chmod 600 /certs/server.key

FROM nginx:1.28-alpine AS runtime
RUN rm /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/nginx.conf
COPY conf.d/default.conf /etc/nginx/conf.d/default.conf
COPY --chmod=755 start.sh /app/start.sh
COPY --from=cert-builder --chown=nginx:nginx /certs/ /etc/nginx/ssl/
USER nginx
EXPOSE 8080 8443
HEALTHCHECK --interval=5s --timeout=3s --retries=5 CMD wget --no-check-certificate -q -O /dev/null https://127.0.0.1:8443/healthz || exit 1
ENTRYPOINT ["/app/start.sh"]
