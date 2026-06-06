FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dokploy-alertmanager ./cmd/dokploy-alertmanager

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/dokploy-alertmanager /dokploy-alertmanager
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/dokploy-alertmanager"]
