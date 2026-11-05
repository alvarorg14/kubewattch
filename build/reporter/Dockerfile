ARG BASE_IMAGE=golang:1.27.3
FROM ${BASE_IMAGE} AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /reporter ./cmd/reporter
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /reporter /reporter
USER 65532:65532
ENTRYPOINT ["/reporter"]
