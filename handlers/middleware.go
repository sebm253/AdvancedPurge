package handlers

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (h *Handler) MiddlewareButtonUser() handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return func(event *handler.InteractionEvent) error {
			messageBuilder := discord.NewMessageCreateBuilder().SetEphemeral(true)
			purge := h.controller.Purge(event.Channel().ID())
			if purge == nil {
				return event.CreateMessage(messageBuilder.
					SetContent("There is no purge being set up.").
					Build())
			}
			if purge.UserID != event.User().ID {
				return event.CreateMessage(messageBuilder.
					SetContent("You cannot interact with purge setups of other users.").
					Build())
			}
			if purge.Running {
				return event.CreateMessage(messageBuilder.
					SetContent("Your purge is already running.").
					Build())
			}
			return next(event)
		}
	}
}
