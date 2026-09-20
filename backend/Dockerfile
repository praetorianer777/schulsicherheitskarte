FROM golang:1.27-alpine AS build
WORKDIR /src
# Dependencies change far less often than the source, so they get their own layer.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -o /out/importer ./cmd/importer \
 && mkdir -p /out/cache

FROM gcr.io/distroless/static-debian12 AS migrate
COPY --from=build /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]

FROM gcr.io/distroless/static-debian12 AS importer
COPY --from=build /out/importer /importer
# Downloads land here. Docker seeds an empty named volume from the image, so
# the directory has to exist with the right owner — the image has no shell to
# fix it up at runtime.
COPY --from=build --chown=nonroot:nonroot /out/cache /data/cache
USER nonroot:nonroot
ENTRYPOINT ["/importer"]
