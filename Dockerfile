FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kapture ./cmd/kapture

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /kapture /usr/local/bin/kapture
# Also create symlink for backward compatibility
RUN ln -s /usr/local/bin/kapture /usr/local/bin/logcatcher

ENTRYPOINT ["kapture"]
