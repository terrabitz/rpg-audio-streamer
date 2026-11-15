package discord

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave"
	"github.com/disgoorg/godave/golibdave"
	discordid "github.com/disgoorg/snowflake/v2"
	"github.com/pion/opus"
)

const opusSamples = 960

// packetQueue absorbs transient UDP, DAVE key-rotation, and gateway lock
// stalls so the mixer's small subscription buffer never overflows; the
// encoder applies backpressure beyond it rather than dropping audio.
const packetQueue = 25

// pcmEncoder accepts the mixer's interleaved float32 stereo PCM and retains
// partial blocks until a complete 20 ms frame is available.
type pcmEncoder struct {
	encoder *opus.Encoder
	samples []float32
	packets chan []byte
	ctx     context.Context
}

func newPCMEncoder(ctx context.Context) (*pcmEncoder, error) {
	encoder, err := opus.NewEncoder(opus.WithSampleRate(48000), opus.WithChannels(2), opus.WithBitrate(128000))
	if err != nil {
		return nil, fmt.Errorf("initialize Discord Opus encoder")
	}
	return &pcmEncoder{encoder: encoder, packets: make(chan []byte, packetQueue), ctx: ctx}, nil
}

func (p *pcmEncoder) Write(data []byte) (int, error) {
	if len(data)%8 != 0 {
		return 0, errors.New("incomplete stereo PCM frame")
	}
	for offset := 0; offset < len(data); offset += 4 {
		sample := math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
		if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) || sample < -1 || sample > 1 {
			return 0, errors.New("invalid stereo PCM sample")
		}
		p.samples = append(p.samples, sample)
		if len(p.samples) == 2*opusSamples {
			packet := make([]byte, voice.MaxOpusFrameSize)
			n, err := p.encoder.EncodeFloat32(p.samples, packet)
			if err != nil {
				return 0, errors.New("Discord Opus encoding failed")
			}
			p.samples = p.samples[:0]
			select {
			case p.packets <- packet[:n]:
			case <-p.ctx.Done():
				return 0, p.ctx.Err()
			}
		}
	}
	return len(data), nil
}

func (*pcmEncoder) Close() error { return nil }

func startVoice(ctx context.Context, cfg Config) (*transport, error) {
	guildID, err := discordid.Parse(cfg.GuildID)
	if err != nil || guildID == 0 {
		return nil, errors.New("invalid Discord server ID")
	}
	channelID, err := discordid.Parse(cfg.ChannelID)
	if err != nil || channelID == 0 {
		return nil, errors.New("invalid Discord voice channel ID")
	}
	voiceErrors := make(chan error, 1)
	refreshSpeaking := make(chan struct{}, 1)
	fail := func() {
		select {
		case voiceErrors <- errors.New("Discord voice connection lost"):
		default:
		}
	}
	var connected atomic.Bool
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client, err := disgo.New(cfg.Token,
		bot.WithLogger(logger),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuilds|gateway.IntentGuildVoiceStates),
			gateway.WithCloseHandler(func(gateway.Gateway, error, bool) { fail() }),
		),
		bot.WithVoiceManagerConfigOpts(
			voice.WithDaveSessionCreateFunc(newDaveSession),
			voice.WithDaveSessionLogger(logger),
			voice.WithConnConfigOpts(
				voice.WithConnGatewayConfigOpts(voice.WithGatewayCloseObserver(func(error) { fail() })),
				voice.WithConnEventHandlerFunc(func(_ voice.Gateway, op voice.Opcode, _ int, _ voice.GatewayMessageData) {
					if op == voice.OpcodeClientsConnect || op == voice.OpcodeDaveExecuteTransition {
						select {
						case refreshSpeaking <- struct{}{}:
						default:
						}
					}
				}),
			),
		),
		bot.WithEventListenerFunc(func(event *events.GuildVoiceStateUpdate) {
			state := event.VoiceState
			if connected.Load() && state.GuildID == guildID && state.UserID == event.Client().ApplicationID &&
				(state.ChannelID == nil || *state.ChannelID != channelID) {
				fail()
			}
		}),
	)
	if err != nil {
		return nil, errors.New("unable to initialize Discord bot")
	}
	closeClient := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Close(closeCtx)
	}
	readyCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := client.OpenGateway(readyCtx); err != nil {
		closeClient()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("unable to open Discord gateway")
	}
	conn := client.VoiceManager.CreateConn(guildID)
	if err := conn.Open(readyCtx, channelID, false, true); err != nil {
		closeClient()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("unable to join Discord voice; check permissions and network access")
	}
	connected.Store(true)
	return startTransport(ctx, discordOutput{conn}, voiceErrors, refreshSpeaking, closeClient)
}

func newDaveSession(logger *slog.Logger, userID godave.UserID, callbacks godave.Callbacks) godave.Session {
	return &daveSession{Session: golibdave.NewSession(logger, userID, callbacks), transitions: make(map[uint16]uint16)}
}

// The released binding does not report disabled DAVE as ready. Track that
// state separately so it cannot remain ready during a new encrypted handshake.
type daveSession struct {
	godave.Session
	disabled    atomic.Bool
	mu          sync.Mutex
	transitions map[uint16]uint16
}

func (s *daveSession) Ready() bool {
	return s.disabled.Load() || s.Session.Ready()
}

func (s *daveSession) OnSelectProtocolAck(version uint16) {
	s.disabled.Store(false)
	s.Session.OnSelectProtocolAck(version)
	s.disabled.Store(version == 0)
}

func (s *daveSession) OnDavePrepareEpoch(epoch int, version uint16) {
	if version != 0 {
		s.disabled.Store(false)
	}
	s.Session.OnDavePrepareEpoch(epoch, version)
}

func (s *daveSession) OnDavePrepareTransition(id, version uint16) {
	if id == 0 {
		s.disabled.Store(false)
		s.Session.OnDavePrepareTransition(id, version)
		s.disabled.Store(version == 0)
		return
	}
	s.mu.Lock()
	s.transitions[id] = version
	s.mu.Unlock()
	s.Session.OnDavePrepareTransition(id, version)
}

func (s *daveSession) OnDaveExecuteTransition(id uint16) {
	s.mu.Lock()
	version, found := s.transitions[id]
	delete(s.transitions, id)
	s.mu.Unlock()
	if found {
		s.disabled.Store(false)
	}
	s.Session.OnDaveExecuteTransition(id)
	if found {
		s.disabled.Store(version == 0)
	}
}

// startTransport paces sends from the mixer subscription itself: each encoded
// packet is sent as soon as it is produced, so no second clock can drift
// against the mixer's 20 ms renderer.
func startTransport(ctx context.Context, output voiceOutput, failures <-chan error, refresh <-chan struct{}, closeOutput func()) (*transport, error) {
	voiceCtx, stopVoice := context.WithCancel(ctx)
	encoder, err := newPCMEncoder(voiceCtx)
	if err != nil {
		stopVoice()
		closeOutput()
		return nil, err
	}
	done := make(chan struct{})
	voiceTransport := &transport{input: encoder, done: done}
	go func() {
		defer close(done)
		defer closeOutput()
		defer stopVoice()
		voiceTransport.err = sendPackets(voiceCtx, output, encoder.packets, failures, refresh, time.Now)
	}()
	return voiceTransport, nil
}

type voiceOutput interface {
	Ready() bool
	Speaking(context.Context) error
	Write([]byte) error
}

type discordOutput struct{ voice.Conn }

func (d discordOutput) Ready() bool {
	return d.DAVE().Ready()
}
func (d discordOutput) Speaking(ctx context.Context) error {
	return d.SetSpeaking(ctx, voice.SpeakingFlagMicrophone)
}
func (d discordOutput) Write(packet []byte) error {
	if err := d.UDP().SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	_, err := d.UDP().Write(packet)
	return err
}

func sendPackets(ctx context.Context, output voiceOutput, packets <-chan []byte, failures <-chan error, refresh <-chan struct{}, now func() time.Time) error {
	ctx, cancel := context.WithCancel(ctx)
	speakingRequests := make(chan struct{}, 1)
	speakingErrors := make(chan error, 1)
	announcerDone := make(chan struct{})
	go func() {
		defer close(announcerDone)
		announceSpeaking(ctx, output, speakingRequests, speakingErrors)
	}()
	defer func() { cancel(); <-announcerDone }()
	announced := false
	var lastSpeaking time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			return err
		case err := <-speakingErrors:
			return err
		case <-refresh:
			lastSpeaking = time.Time{}
		case packet := <-packets:
			// Frames sent while DAVE holds would be unreadable by E2EE
			// receivers; they are discarded, not queued, so a long key
			// exchange cannot back up into the mixer.
			if !output.Ready() || len(packet) == 0 {
				continue
			}
			at := now()
			if !announced {
				// Receivers ignore RTP until a speaking event maps the SSRC.
				if err := speak(ctx, output); err != nil {
					return err
				}
				announced, lastSpeaking = true, at
			} else if lastSpeaking.IsZero() || at.Sub(lastSpeaking) >= 5*time.Second {
				// Late joiners learn the SSRC only from speaking events; the
				// gateway lock can be held during MLS commits, so refreshes run
				// beside the audio path instead of stalling it.
				select {
				case speakingRequests <- struct{}{}:
				default:
				}
				lastSpeaking = at
			}
			if err := output.Write(packet); err != nil {
				return errors.New("Discord voice audio send failed")
			}
		}
	}
}

func announceSpeaking(ctx context.Context, output voiceOutput, requests <-chan struct{}, failures chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-requests:
			if err := speak(ctx, output); err != nil {
				failures <- err
				return
			}
		}
	}
}

func speak(ctx context.Context, output voiceOutput) error {
	speakingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := output.Speaking(speakingCtx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Discord voice speaking setup failed")
	}
	return nil
}

var _ io.WriteCloser = (*pcmEncoder)(nil)
