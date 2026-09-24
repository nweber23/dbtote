FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/dbtote ./cmd/dbtote

FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      default-mysql-client postgresql-client ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/dbtote /usr/local/bin/dbtote
ENTRYPOINT ["dbtote"]