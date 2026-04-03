FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/egudoc ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/bin/egudoc /egudoc
USER nonroot:nonroot
EXPOSE 8090
ENTRYPOINT ["/egudoc"]
