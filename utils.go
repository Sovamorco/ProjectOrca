package main

import (
	"github.com/bwmarrin/discordgo"
	"regexp"
)

var urlRx = regexp.MustCompile(`https?://(?:www\.)?.+`)

func atMostAbs[T int | float32 | float64](num, clamp T) T {
	if num < 0 {
		return max(num, -clamp)
	}
	return min(num, clamp)
}

func (s *State) respondAck(i *discordgo.InteractionCreate) error {
	err := s.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			err = nil
		}
	}
	return err
}

func (s *State) respondSimpleMessage(i *discordgo.InteractionCreate, message string) error {
	return s.respond(i, &discordgo.InteractionResponseData{
		Content: message,
	})
}

func (s *State) respondColoredEmbed(i *discordgo.InteractionCreate, color Color, name, message string) error {
	embed := discordgo.MessageEmbed{
		Color: int(color),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   name,
				Value:  message,
				Inline: false,
			},
		},
	}
	return s.respond(i, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			&embed,
		},
	})
}

func (s *State) respondSimpleEmbed(i *discordgo.InteractionCreate, name, message string) error {
	return s.respondColoredEmbed(i, DefaultColor, name, message)
}

func (s *State) respond(i *discordgo.InteractionCreate, response *discordgo.InteractionResponseData) error {
	err := s.Session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: response,
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			_, err = s.Session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content:         &response.Content,
				Components:      &response.Components,
				Embeds:          &response.Embeds,
				Files:           response.Files,
				AllowedMentions: response.AllowedMentions,
			})
		}
	}
	return err
}

func empty[T any](c chan T) {
	for len(c) != 0 {
		select {
		case <-c:
		default:
		}
	}
}
