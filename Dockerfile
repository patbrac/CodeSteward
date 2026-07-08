# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.24-alpine AS build

ARG VERSION=0.1.0-dev
ARG COMMIT=none
ARG DATE=unknown

ENV VERSION_PKG=github.com/codesteward-ai/codesteward/internal/version

WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w \
	-X ${VERSION_PKG}.Version=${VERSION} \
	-X ${VERSION_PKG}.Commit=${COMMIT} \
	-X ${VERSION_PKG}.Date=${DATE}" \
	-o /out/codesteward ./cmd/codesteward

# ---- final stage ----
FROM alpine:3.20

# ca-certificates for HTTPS API calls; git because the scan shells out to
# git in CI checkouts.
RUN apk add --no-cache ca-certificates git \
	&& adduser -D -H -u 10001 codesteward

COPY --from=build /out/codesteward /usr/local/bin/codesteward

USER codesteward

ENTRYPOINT ["codesteward"]
