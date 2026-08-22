FROM golang:1.25.13-alpine@sha256:1e0126852075c9c60731c8ba49088448b91f63e2aed97ca9d1a9791622a05946 AS build

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/hotelmate-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/seed-demo ./cmd/seed-demo
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/purge-documents ./cmd/purge-documents
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/purge-messages ./cmd/purge-messages
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /out/hotelmate ./cmd/hotelmate

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates postgresql16-client tzdata
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/hotelmate-api /app/hotelmate-api
COPY --from=build /out/seed-demo /app/seed-demo
COPY --from=build /out/purge-documents /app/purge-documents
COPY --from=build /out/purge-messages /app/purge-messages
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/hotelmate /app/hotelmate
RUN mkdir -p /app/uploads && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/hotelmate-api"]
