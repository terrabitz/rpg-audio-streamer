# TODO

Tasks to complete before releasing to my players:
- [X] Fix reconnection bug
- [X] Add player track type mixer
- [X] Add GM track type mixer
- [X] Infrastructure setup

Tasks to complete before GA of self-hostable:
- [X] Automate Docker image build
- [X] Add example Docker and Docker Compose configurations
- [X] File metadata editing
- [ ] Save table audio state persistently on server (i.e. volumes)
- [ ] Multiple tables
- [ ] Better file management
- [ ] Improve initial setup experience

Tasks to complete before GA of hosted:
- [ ] Landing page
- [ ] Deployment/infrastructure pipelines
- [ ] Observability and analytics
- [ ] Update authentication to use identity proxy
- [ ] Add per-user storage quotas
- [ ] Use cloud storage
- [ ] Improve file management security (e.g. enforce size limits, validate content type, limits/timeouts to avoid DoS, process isolation)
- [ ] Monetization (e.g. subscription for extra storage)

Additional tasks:
- [X] API docs
- [X] Tiled track layout
- [X] Make the player view URL the same as the join URL
- [ ] Customizable track order
- [ ] Mobile layout support for GMs
- [ ] Turn frontend into a PWA
- [ ] Integration with other streaming providers (YouTube, Spotify)
- [ ] Discord integration
- [ ] Add automatic updates for the GIF demo on the README
- [ ] Add "always start track from beginning" option
- [ ] Add visibility for players whether a track is playing or not
- [ ] Add "website has been updated" indicator so players can refresh their browsers more easily
- [ ] Add track start/end customizer
- [ ] Add cross-fade looping (i.e. the beginning of the track cross fades with the end of the track)
- [ ] Add customizable looping. Since many songs start slower and ramp up, the user should be able to configure where the track should loop, so the energy stays consistent. This may or may not be the same as the track start/end.

Bugs:
- [X] Fix issue with audio bounce when master volume is adjusted while fading
- [X] Fix websocket disconnect issue on prod site
- [ ] Fix issue with track reset in player view when navigating away
- [ ] Fix mixing issue. When we send track info, we should always send the volume relative to 100%, not relative to the GM master volume. This will allow players to make it as loud as they want, while still ensuring the mixing is correct.
- [ ] Shorten websocket reconnect time
