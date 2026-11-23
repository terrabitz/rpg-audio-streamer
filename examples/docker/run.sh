docker run -d \
  --name rpg-audio-streamer \
  -p 8080:80 \
  -v $PWD/data:/app/data \
  ghcr.io/terrabitz/rpg-audio-streamer:latest
