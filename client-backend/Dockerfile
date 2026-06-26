FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/relay-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/rack-agent ./cmd/rack-agent
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/exos-hypervisor-agent ./cmd/hypervisor-agent
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/wireguard-controller ./cmd/wireguard-controller

FROM alpine:3.21 AS api

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/relay-api /usr/local/bin/relay-api
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY db/migrations ./db/migrations

EXPOSE 8080 9090

CMD ["sh", "-c", "goose -dir ./db/migrations postgres \"$DATABASE_URL\" up && exec relay-api"]

FROM alpine:3.21 AS rack-agent

RUN apk add --no-cache ca-certificates kubectl

WORKDIR /app

COPY --from=build /out/rack-agent /usr/local/bin/rack-agent

CMD ["rack-agent"]

FROM alpine:3.21 AS hypervisor-agent

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/exos-hypervisor-agent /usr/local/bin/exos-hypervisor-agent

CMD ["exos-hypervisor-agent"]

FROM alpine:3.21 AS wireguard-controller

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/wireguard-controller /usr/local/bin/wireguard-controller

CMD ["wireguard-controller"]

FROM api AS default
