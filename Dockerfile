FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w -extldflags '-static'" -o /app/webui-be ./cmd/webui-be


FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/webui-be /app/webui-be
RUN mkdir -p /data

EXPOSE 8080

ENTRYPOINT ["/app/webui-be"]
