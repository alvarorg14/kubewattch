ARG BASE_IMAGE=golang:1.27.3
FROM ${BASE_IMAGE} AS build
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 go build -o /simulator ./test/e2e/report-simulator
FROM gcr.io/distroless/static:nonroot
COPY --from=build /simulator /simulator
USER 65532:65532
ENTRYPOINT ["/simulator"]
