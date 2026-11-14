package e2e

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/thomas-vilte/dave-go/session"
)

// listener is a second Discord bot that joins the voice channel and analyzes
// what the streamer bot sends.
type listener struct {
	client  *bot.Client
	conn    voice.Conn
	monitor *monitor
	closed  sync.Once
}

func startListener(ctx context.Context, config liveConfig) (*listener, error) {
	l := &listener{monitor: newMonitor(decodeOpus)}
	client, err := disgo.New(config.ListenerToken,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds|gateway.IntentGuildVoiceStates)),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagVoiceStates)),
		bot.WithVoiceManagerConfigOpts(voice.WithDaveSessionCreateFunc(joinerSession(session.CreateFunc()))),
		bot.WithEventListenerFunc(func(e *events.GuildVoiceStateUpdate) {
			if e.VoiceState.UserID == config.StreamerID {
				l.monitor.setPresent(inChannel(e.VoiceState, config.ChannelID))
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create listener bot: %w", err)
	}
	l.client = client
	if err := client.OpenGateway(ctx); err != nil {
		client.Close(context.Background())
		return nil, fmt.Errorf("open listener gateway: %w", err)
	}

	l.conn = client.VoiceManager.CreateConn(config.GuildID)
	l.conn.SetOpusFrameReceiver(streamerFrames{streamerID: config.StreamerID, monitor: l.monitor})
	if err := l.conn.Open(ctx, config.ChannelID, true, false); err != nil {
		l.Close()
		return nil, fmt.Errorf("join voice channel: %w", err)
	}
	if state, ok := client.Caches.VoiceState(config.GuildID, config.StreamerID); ok {
		l.monitor.setPresent(inChannel(state, config.ChannelID))
	}
	return l, nil
}

func (l *listener) Latest() Audio {
	return l.monitor.latest()
}

func (l *listener) Close() {
	l.closed.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if l.conn != nil {
			l.conn.Close(ctx)
		}
		l.client.Close(ctx)
	})
}

func inChannel(state discord.VoiceState, channelID snowflake.ID) bool {
	return state.ChannelID != nil && *state.ChannelID == channelID
}

type streamerFrames struct {
	streamerID snowflake.ID
	monitor    *monitor
}

func (s streamerFrames) ReceiveOpusFrame(userID snowflake.ID, packet *voice.Packet) error {
	if userID == s.streamerID {
		s.monitor.receive(packet.Opus, time.Now())
	}
	return nil
}

func (streamerFrames) CleanupUser(snowflake.ID) {}

func (streamerFrames) Close() {}
