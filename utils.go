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

func respondAck(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			err = nil
		}
	}
	return err
}

func respondSimpleMessage(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return respond(s, i, &discordgo.InteractionResponseData{
		Content: message,
	})
}

func respondColoredEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, color Color, name, message string) error {
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
	return respond(s, i, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			&embed,
		},
	})
}

func respondSimpleEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, name, message string) error {
	return respondColoredEmbed(s, i, DefaultColor, name, message)
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, response *discordgo.InteractionResponseData) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: response,
	})
	if re, ok := err.(*discordgo.RESTError); ok {
		if re.Message.Code == discordgo.ErrCodeInteractionHasAlreadyBeenAcknowledged {
			_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
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
