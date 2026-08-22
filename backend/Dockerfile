FROM golang:1.27.0-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build

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

FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d
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
