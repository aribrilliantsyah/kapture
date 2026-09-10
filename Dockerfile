FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /logcatcher ./cmd/logcatcher

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /logcatcher /usr/local/bin/logcatcher

ENTRYPOINT ["logcatcher"]
