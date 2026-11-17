package domain

import (
	"fmt"
	"strconv"
)

type Destination struct {
	GuildID     string `json:"guildID"`
	ChannelID   string `json:"channelID"`
	GuildName   string `json:"guildName,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
}

type DiscordStatus struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
	*Destination
	Error string `json:"error,omitempty"`
}

func (d Destination) Validate() error {
	if !ValidDiscordID(d.GuildID) || !ValidDiscordID(d.ChannelID) {
		return fmt.Errorf("select a valid Discord server and voice channel")
	}
	return nil
}

func ValidDiscordID(value string) bool {
	if value == "" || len(value) > 20 || value[0] == '0' {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}
