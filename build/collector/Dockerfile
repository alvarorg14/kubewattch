ARG BASE_IMAGE=golang:1.27.3
FROM ${BASE_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /collector ./cmd/collector
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /collector /collector
USER 65532:65532
ENTRYPOINT ["/collector"]
