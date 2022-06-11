ARG GO_VERSION=1.18.3

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN apk --no-cache add ca-certificates
RUN CGO_ENABLED=0 go build -o /app ./cmd/api

FROM scratch

COPY --from=builder /app /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/app", "--cert=/var/lib/certs/localhost.cert.pem", "--key=/var/lib/certs/localhost.key.pem"]
