FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /james-agent ./cmd/james-agent

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /james-agent /usr/local/bin/james-agent

RUN mkdir -p /root/.james-agent/workspace

VOLUME ["/root/.james-agent"]

EXPOSE 18790 9876 9886

ENTRYPOINT ["james-agent"]
CMD ["gateway"]
