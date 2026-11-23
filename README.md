# RPG Audio Streamer

A simple synchronized soundboard designed for tabletop RPGs, allowing game masters to manage and stream background music and sound effects during gameplay sessions.

![The GM switches music in the soundboard while a player listens in a second browser](./media/audio-playing.gif)

## Features

- **Real-time music streaming** - All playing tracks on the GM side are immediately synced to players
- **Optional Discord integration** - Stream all sounds to a Discord bot channel
- **Seamless playback** - Automatic fades and advanced track looping controls make everything seamless for your players

## Quick Start

You easily get started using the Docker image

```bash
# Create the data direcotory
mkdir -p data/uploads

# Run the image
docker run -d \
   --name rpg-audio-streamer \
   -p 8080:80 \
   -v $PWD/data:/app/data \
   ghcr.io/terrabitz/rpg-audio-streamer:latest

# Get the setup code from the logs
docker logs rpg-audio-streamer
```

See [INSTALLATION.md](./INSTALLATION.md) for more installation methods

## Development

See [CONTRIBUTORS.md](./CONTRIBUTORS.md) for details

## License

MIT License - See [LICENSE](./LICENSE) file for details
