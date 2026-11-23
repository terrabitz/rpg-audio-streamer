# Create the data directory
mkdir -p data/uploads

# Run the image
docker run -d \
   --name rpg-audio-streamer \
   -p 8080:80 \
   -v $PWD/data:/app/data \
   ghcr.io/terrabitz/rpg-audio-streamer:latest

# Get the setup code from the logs
docker logs rpg-audio-streamer
