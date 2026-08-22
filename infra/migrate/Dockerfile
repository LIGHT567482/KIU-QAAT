FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY backend/api-gateway/go.mod backend/api-gateway/go.sum ./
RUN go mod download
COPY backend/api-gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /migrate ./cmd/migrate

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata postgresql-client
COPY --from=builder /migrate /migrate
COPY db/migrations /migrations
COPY db/seeds/010_kiu_institution.sql /seeds/010_kiu_institution.sql
COPY infra/migrate/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
ENTRYPOINT ["/docker-entrypoint.sh"]
