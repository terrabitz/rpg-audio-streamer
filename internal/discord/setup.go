package discord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"

	"github.com/disgoorg/disgo/rest"
)

type Choice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Setup struct {
	InstallURL string   `json:"installURL"`
	Guilds     []Choice `json:"guilds"`
}

func (s *Service) Setup(ctx context.Context) (Setup, error) {
	var application struct {
		ID string `json:"id"`
	}
	if err := s.get(ctx, "/oauth2/applications/@me", &application); err != nil {
		return Setup{}, err
	}
	if !snowflake(application.ID) {
		return Setup{}, fmt.Errorf("discord returned an invalid application ID")
	}
	guilds := []Choice{}
	after := ""
	for {
		var page []Choice
		path := "/users/@me/guilds?limit=200"
		if after != "" {
			path += "&after=" + after
		}
		if err := s.get(ctx, path, &page); err != nil {
			return Setup{}, err
		}
		guilds = append(guilds, page...)
		if len(page) < 200 {
			break
		}
		last := page[len(page)-1].ID
		if !snowflake(last) || last == after {
			return Setup{}, fmt.Errorf("discord returned an invalid server-list cursor")
		}
		after = last
	}
	sort.Slice(guilds, func(i, j int) bool { return guilds[i].Name < guilds[j].Name })
	values := url.Values{
		"client_id": {application.ID}, "scope": {"bot"}, "permissions": {"3146752"},
		"integration_type": {"0"},
	}
	return Setup{InstallURL: "https://discord.com/oauth2/authorize?" + values.Encode(), Guilds: guilds}, nil
}

func (s *Service) Channels(ctx context.Context, guildID string) ([]Choice, error) {
	if !snowflake(guildID) {
		return nil, fmt.Errorf("select a valid Discord server")
	}
	var channels []struct {
		Choice
		Type int `json:"type"`
	}
	if err := s.get(ctx, "/guilds/"+guildID+"/channels", &channels); err != nil {
		return nil, err
	}
	choices := []Choice{}
	for _, channel := range channels {
		if channel.Type == 2 {
			choices = append(choices, channel.Choice)
		}
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })
	return choices, nil
}

func (s *Service) Resolve(ctx context.Context, target Destination) (Destination, error) {
	if err := target.Validate(); err != nil {
		return Destination{}, err
	}
	var guild Choice
	if err := s.get(ctx, "/guilds/"+target.GuildID, &guild); err != nil {
		return Destination{}, err
	}
	channels, err := s.Channels(ctx, target.GuildID)
	if err != nil {
		return Destination{}, err
	}
	for _, channel := range channels {
		if channel.ID == target.ChannelID {
			return Destination{GuildID: guild.ID, GuildName: guild.Name, ChannelID: channel.ID, ChannelName: channel.Name}, nil
		}
	}
	return Destination{}, fmt.Errorf("selected channel is not a regular voice channel in this Discord server")
}

func (s *Service) get(ctx context.Context, path string, out any) error {
	endpoint, err := url.Parse(path)
	if err != nil {
		return err
	}
	s.apiMu.Lock()
	if s.apiClient == nil {
		s.apiClient = rest.NewClient(s.cfg.Token,
			rest.WithURL(s.apiURL), rest.WithHTTPClient(s.http),
			rest.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		)
	}
	client := s.apiClient
	s.apiMu.Unlock()
	opts := []rest.RequestOpt{rest.WithCtx(ctx)}
	for key, values := range endpoint.Query() {
		opts = append(opts, rest.WithQueryParam(key, values[0]))
	}
	if err := client.Do(rest.NewEndpoint(http.MethodGet, endpoint.Path).Compile(nil), nil, out, opts...); err != nil {
		var response *rest.Error
		if errors.As(err, &response) && response.Response != nil {
			return fmt.Errorf("discord setup request failed (HTTP %d); check the bot token, installation, and channel permissions", response.Response.StatusCode)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("discord setup request failed; check network access and try again")
	}
	return nil
}
