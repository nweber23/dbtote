FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/dbtote ./cmd/dbtote

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      gnupg curl ca-certificates default-mysql-client postgresql-client sqlite3 \
    && curl -fsSL https://pgp.mongodb.com/server-8.0.asc | gpg --dearmor -o /usr/share/keyrings/mongodb-server-8.0.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/mongodb-server-8.0.gpg] https://repo.mongodb.org/apt/debian bookworm/mongodb-org/8.0 main" \
         > /etc/apt/sources.list.d/mongodb-org-8.0.list \
    && apt-get update && apt-get install -y --no-install-recommends mongodb-database-tools \
    && apt-get purge -y gnupg curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/dbtote /usr/local/bin/dbtote
ENTRYPOINT ["dbtote"]