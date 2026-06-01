FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/seedance-direct-proxy ./cmd/seedance-direct-proxy

FROM alpine:3.20

RUN adduser -D -H -u 10001 appuser
COPY --from=builder /out/seedance-direct-proxy /usr/local/bin/seedance-direct-proxy

ENV PORT=3000
EXPOSE 3000

USER appuser
ENTRYPOINT ["/usr/local/bin/seedance-direct-proxy"]
