FROM golang:1.24-bookworm AS base
ENV PATH="/usr/local/go/bin:${PATH}" \
    GOFLAGS="-buildvcs=false"
WORKDIR /app
COPY go.mod ./
RUN go mod download

FROM base AS dev
WORKDIR /app
COPY . .
EXPOSE 8080
CMD ["go", "run", "./cmd/server"]

FROM base AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/bank-of-vivaldi ./cmd/server

FROM gcr.io/distroless/base-debian12 AS runtime
WORKDIR /app
COPY --from=build /out/bank-of-vivaldi /app/bank-of-vivaldi
ENV HTTP_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/app/bank-of-vivaldi"]
