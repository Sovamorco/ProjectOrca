FROM golang:1.21-rc-alpine AS builder
LABEL stage=builder

RUN apk add build-base git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY extractor extractor
COPY healthcheck healthcheck
COPY migrations migrations
COPY models models
COPY proto proto
COPY spotify spotify
COPY store store
COPY utils utils
COPY vk vk
COPY ytdl ytdl
COPY main.go config.go ./

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

RUN apk add ffmpeg yt-dlp opus opus-dev

WORKDIR /src
COPY --from=builder /src/bin/ ./bin
COPY config.yaml ./

ENV ORCA_HEALTH_ADDRESS=localhost:8590

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=4 \
    CMD /src/bin/healthcheck

USER appuser:appuser

ENTRYPOINT ["/src/bin/orca"]
