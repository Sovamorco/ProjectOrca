FROM golang:1.22-alpine AS builder
LABEL stage=builder

RUN apk add --no-cache build-base git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go mod vendor && go mod verify
RUN GOOS=linux GOARCH=amd64 GOMAXPROCS=1 go build -gcflags="all=-c=1" -ldflags="-w -s" -o /src/bin/healthcheck ./healthcheck
RUN GOOS=linux GOARCH=amd64 GOMAXPROCS=1 go build -gcflags="all=-c=1" -ldflags="-w -s" -o /src/bin/orca .


FROM alpine:latest
# Create appuser.
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

RUN apk add --no-cache ffmpeg opus opus-dev

WORKDIR /src
COPY --from=builder /src/bin/ ./bin
COPY config.yaml ./

# don't want to cache yt-dlp specifically so put it here
RUN apk add --no-cache yt-dlp

ENV ORCA_HEALTH_ADDRESS=localhost:8590

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=4 \
    CMD /src/bin/healthcheck

USER appuser:appuser

ENTRYPOINT ["/src/bin/orca"]
