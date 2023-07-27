FROM golang:1.21-rc-alpine AS builder
LABEL stage=builder

RUN apk add build-base git

WORKDIR /src
COPY extractor extractor
COPY migrations migrations
COPY models models
COPY proto proto
COPY spotify spotify
COPY store store
COPY utils utils
COPY vk vk
COPY ytdl ytdl
COPY config.go go.mod go.sum main.go ./

RUN go mod vendor  \
    && go mod verify \
    && GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /src/bin/orca .


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

RUN apk add ffmpeg yt-dlp

WORKDIR /src
COPY --from=builder /src/bin/ ./bin
COPY config.yaml ./
USER appuser:appuser

ENTRYPOINT ["sh", "-c", "exec /src/bin/orca"]
