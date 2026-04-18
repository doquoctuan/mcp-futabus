# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN set -eux; \
        GOOS_VALUE="${TARGETOS:-$(go env GOOS)}"; \
        GOARCH_VALUE="${TARGETARCH:-$(go env GOARCH)}"; \
        CGO_ENABLED=0 GOOS="$GOOS_VALUE" GOARCH="$GOARCH_VALUE" \
            go build -trimpath -ldflags="-s -w" -o /out/mcp-futabus .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/mcp-futabus /app/mcp-futabus

EXPOSE 8080
ENTRYPOINT ["/app/mcp-futabus", "--http", ":8080"]
