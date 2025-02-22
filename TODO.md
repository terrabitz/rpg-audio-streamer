# TODO

Tasks to complete before releasing to my players:
- [X] Fix reconnection bug
- [X] Add player track type mixer
- [X] Add GM track type mixer
- [X] Infrastructure setup

Tasks to complete before GA of self-hostable:
- [ ] Better file management
- [ ] Improve initial setup experience
- [ ] Automate Docker image build
- [ ] Add example Docker and Docker Compose configurations

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
- [ ] Customizable tiled track layout
- [ ] Mobile layout support
- [ ] Turn frontend into a PWA
- [ ] API docs
- [ ] Integration with other streaming providers (YouTube, Spotify)
- [ ] Discord integration
- [ ] Add automatic updates for the GIF demo on the README
- [ ] Better navigation (e.g. restricting access to login if we're authenticated, removing navigation to most pages as a player)

Bugs:
- [X] Fix issue with audio bounce when master volume is adjusted while fading
- [ ] Fix issue with track reset in player view when navigating away
