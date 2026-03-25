FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o synocli ./cmd/synocli

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /build/synocli /usr/local/bin/synocli

ENTRYPOINT ["synocli"]
