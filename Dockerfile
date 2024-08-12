FROM golang:alpine as builder
WORKDIR /src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -ldflags "-s -w" -o go-rss-bridge cmd/main.go

FROM gcr.io/distroless/base-debian12:nonroot
ENV GIN_MODE release
WORKDIR /app
COPY --from=builder /src/app/go-rss-bridge ./go-rss-bridge
ENTRYPOINT ["./go-rss-bridge"]
