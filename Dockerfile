# syntax=docker/dockerfile:1

# ---- build stage -----------------------------------------------------------
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/beziersvc ./cmd/server

# ---- runtime stage ---------------------------------------------------------
FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/beziersvc /usr/local/bin/beziersvc
USER app
EXPOSE 8080
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/v1/config || exit 1
ENTRYPOINT ["beziersvc"]
