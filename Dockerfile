FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-X 'synocli/internal/cli.buildVersion=${VERSION}' -X 'synocli/internal/cli.buildCommit=${COMMIT}' -X 'synocli/internal/cli.buildDate=${BUILD_DATE}'" \
    -o synocli ./cmd/synocli

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /build/synocli /usr/local/bin/synocli

ENTRYPOINT ["synocli"]
