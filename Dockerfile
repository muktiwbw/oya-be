FROM golang:1.26.10-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/oya ./cmd/oya

FROM alpine:3.22
RUN addgroup -g 10001 oya && adduser -D -u 10001 -G oya oya
COPY --from=build /out/oya /usr/local/bin/oya
USER 10001:10001
ENV OYA_HTTP_ADDR=:8080 DB_HOST=localhost DB_PORT=15432 DB_NAME=oya DB_USER=postgres DB_PASSWORD=postgres
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s CMD ["/usr/local/bin/oya", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/oya"]
