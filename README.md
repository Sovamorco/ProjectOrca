# Orca

Highly involved, language-independent, resilient and relatively thin server for Discord music playback written in Go.

## Client

Orca uses gRPC and [Protocol Buffers](https://protobuf.dev/) for client-server communication. That means you can generate client files by running `protoc` for your specific language.

You can see generic overview of possible functions in `proto/orca.proto`.

Taskfile includes protoc command for Python: `task proto:python`, which generates Python files in `proto_python` directory.

## Development

This project uses [Task](https://taskfile.dev) to run most common tasks locally.

Local development environment requires Redis and Postgres to be running with credentials specified in `config.dev.yaml`.

To run the server - just do `task run`.

## Production

Running in docker is recommended.

To operate Orca requires Redis and Postgres to be running.

### Configuration

Config structure can be seen in `config.dev.yaml`.

To pass your own configuration, mount the config file in the container and pass the CONFIG_PATH environment variable with the path of the mounted config file.

## Optional modules

Orca provides optional VK and Spotify playback, both of which require credentials.

### VK

To enable VK playback, config should contain token with audio.get permissions:
```
vk:
  token: <YOUR_TOKEN>
```

### Spotify

To enable Spotify playback, config should contain ClientID and ClientSecret for a [Spotify app](https://developer.spotify.com/documentation/web-api/concepts/apps):
```
spotify:
  client_id: <YOUR_CLIENT_ID>
  client_secret: <YOUR_CLIENT_SECRET>
```

## Features

- **High availability** - you can run multiple instances of the bot and they will automatically substitute each other in case any of them fail
- **Voice conneciton management** - bot manages voice connections for you, giving you simple controls for connecting/disconnecting
- **Multitude of sources** - bot supports any source that can be fetched by [yt-dlp](https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md) and any format that can be processed by [ffmpeg](https://ffmpeg.org/ffmpeg-formats.html), which sums up to pretty much anything in existence
