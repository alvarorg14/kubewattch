ARG BASE_IMAGE=golang:1.27.3
FROM ${BASE_IMAGE} AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /simulator ./test/e2e/tapo-simulator
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /simulator /simulator
USER 65532:65532
ENTRYPOINT ["/simulator"]
