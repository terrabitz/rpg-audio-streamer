# rpg-audio-streamer

## Commands

Stream a file

```bash
curl "http://localhost:8080/api/v1/stream/Test.mp3" -o /tmp/temp_audio.mp3 && afplay /tmp/temp_audio.mp3
```

Upload a file

```bash
curl -X POST http://localhost:8080/api/v1/files -F "files=@tmp/Test.mp3"
```

Delete a file

```bash
curl -X DELETE "http://localhost:8080/api/v1/files/Test.mp3"
```
