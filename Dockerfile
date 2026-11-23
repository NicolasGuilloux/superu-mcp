# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}" -o /out/superu-mcp ./cmd/superu-mcp && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags "-s -w" -o /out/coursesu ./cmd/coursesu && \
    mkdir -p /out/config/coursesu

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/superu-mcp /out/coursesu /usr/local/bin/
# Owned by nonroot so a named volume mounted here starts writable.
COPY --from=build --chown=65532:65532 /out/config /home/nonroot/.config
# The session lives in $HOME/.config/coursesu: mount a volume there.
ENV HOME=/home/nonroot \
    COURSESU_FLARESOLVERR_URL=http://flaresolverr:8191/v1 \
    MCP_ADDR=:8080
VOLUME ["/home/nonroot/.config/coursesu"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/superu-mcp"]
# Default: Streamable HTTP on /mcp and SSE on /sse. Use "mcp" alone for stdio.
CMD ["mcp", "http"]
