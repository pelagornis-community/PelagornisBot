FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /pelagornisbot ./cmd/pelagornisbot
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /pelagornisbot /pelagornisbot
COPY --from=build /src/assets /assets
USER nonroot:nonroot
ENTRYPOINT ["/pelagornisbot"]
