package handlers

import (
	"errors"
	"log/slog"
	"slices"
	"strconv"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
	"github.com/lmittmann/tint"
)

var (
	endMessageFunc = func(endID snowflake.ID) func(discord.Message) bool {
		return func(message discord.Message) bool {
			return message.ID == endID
		}
	}
)

func (h *Handler) HandlePurge(_ discord.SlashCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	purge := h.controller.Purge(event.Channel().ID())
	if purge == nil {
		h.controller.CreatePurge(event.Channel().ID(), event.User().ID)

		return event.CreateMessage(messageBuilder.
			SetContent("Would you like to run a simple or an advanced purge?").
			AddActionRow(
				discord.NewPrimaryButton("Simple purge", "/purge/simple"),
				discord.NewPrimaryButton("Advanced purge (range)", "/purge/advanced"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if purge.UserID == event.User().ID {
		return event.CreateMessage(messageBuilder.
			SetContent("You already have a purge setup running.").
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("This channel already has a purge setup running by <@%d>.", purge.UserID).
		Build())
}

func (h *Handler) HandleSimple(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	return nil
}

func (h *Handler) HandleAdvanced(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	return event.Modal(discord.NewModalCreateBuilder().
		SetTitle("Enter how many messages can be purged at once").
		SetCustomID("purge").
		AddActionRow(
			discord.NewShortTextInput("limit", "Limit of messages to purge at once").
				WithRequired(true).
				WithMaxLength(3)).
		Build())
}

func (h *Handler) HandleCancel(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	h.controller.RemovePurge(event.Channel().ID())
	return event.UpdateMessage(discord.NewMessageUpdateBuilder().
		SetContent("Alright, purge has been canceled.").
		ClearContainerComponents().
		Build())
}

func (h *Handler) HandleLimit(event *handler.ModalEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	amount := event.Data.Text("limit")
	i, err := strconv.Atoi(amount)
	if err != nil || i <= 1 || i > 100 {
		return event.CreateMessage(messageBuilder.
			SetContent("Provide a number between 2 and 100.").
			Build())
	}
	h.controller.SetBulkLimit(event.Channel().ID(), i)

	return event.CreateMessage(messageBuilder.
		SetContent(`Set a starting point by right clicking a message and hitting "**Set as start**".`).
		AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleStart(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	jumpURL := data.TargetMessage().JumpURL()
	if purge.StartID == 0 {
		if ok := h.controller.SetStartID(purge, data.TargetID()); !ok {
			return event.CreateMessage(messageBuilder.
				SetContent("Message cannot be older than 2 weeks.").
				AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
				Build())
		}
		return event.CreateMessage(messageBuilder.
			SetContentf(`Start message has been set to %s. Now, specify a range and select your end message by right clicking a message and hitting "**Set as end**".`, jumpURL).
			AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if purge.StartID == data.TargetID() {
		return event.CreateMessage(messageBuilder.
			SetContent("This message already is the start message.").
			AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("You have already selected a [start message](%s). Do you want to use [this message](%s) as the start instead?",
			discord.MessageURL(*event.GuildID(), channelID, purge.StartID),
			jumpURL).
		AddActionRow(
			discord.NewPrimaryButton("No, keep the previous one.", "/purge/start-change/keep"),
			discord.NewPrimaryButton("Yes, change start to this one.", "/purge/start-change/"+data.TargetID().String()),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleStartKeep(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Alright, keeping [the current message](%s) as the start.", discord.MessageURL(*event.GuildID(), channelID, purge.StartID)).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleStartChange(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	messageBuilder := discord.NewMessageCreateBuilder()
	newID := snowflake.MustParse(event.Vars["new-id"])
	if newID == purge.StartID {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot set the start message to the end message.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if ok := h.controller.SetStartID(purge, newID); !ok {
		return event.CreateMessage(messageBuilder.
			SetContent("Message cannot be older than 2 weeks.").
			AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Alright, start message has been set to [this message](%s).", discord.MessageURL(*event.GuildID(), channelID, purge.StartID)).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleEnd(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	if purge.StartID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the start message first.").
			Build())
	}
	jumpURL := data.TargetMessage().JumpURL()
	if purge.EndID == 0 {
		if data.TargetID() == purge.StartID {
			return event.CreateMessage(messageBuilder.
				SetContent("Cannot set the end message to the start message.").
				AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
				Build())
		}
		if ok := h.controller.SetEndID(purge, data.TargetID()); !ok {
			return event.CreateMessage(messageBuilder.
				SetContent("Messages cannot be older than 2 weeks.").
				AddActionRow(discord.NewDangerButton("Cancel purge", "/purge/cancel")).
				Build())
		}
		return event.CreateMessage(messageBuilder.
			SetContentf("End message has been set to %s.", jumpURL).
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if purge.EndID == data.TargetID() {
		return event.CreateMessage(messageBuilder.
			SetContent("This message already is the end message.").
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("You have already selected an [end message](%s). Do you want to use [this message](%s) as the end instead?",
			discord.MessageURL(*event.GuildID(), channelID, purge.StartID),
			jumpURL).
		AddActionRow(
			discord.NewPrimaryButton("No, keep the previous one.", "/purge/end-change/keep"),
			discord.NewPrimaryButton("Yes, change end to this one.", "/purge/end-change/"+data.TargetID().String()),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleEndKeep(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	return event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContentf("Alright, keeping [the current message](%s) as the end.", discord.MessageURL(*event.GuildID(), channelID, purge.EndID)).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleEndChange(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	channelID := event.Channel().ID()
	purge := h.controller.Purge(channelID)
	messageBuilder := discord.NewMessageCreateBuilder()
	newID := snowflake.MustParse(event.Vars["new-id"])
	if newID == purge.StartID {
		return event.CreateMessage(messageBuilder.
			SetContent("Cannot set the end message to the start message.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if ok := h.controller.SetEndID(purge, newID); !ok {
		return event.CreateMessage(messageBuilder.
			SetContent("Message cannot be older than 2 weeks.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Alright, end message has been set to [this message](%s).", discord.MessageURL(*event.GuildID(), channelID, purge.EndID)).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleExclude(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	purge := h.controller.Purge(event.Channel().ID())
	if purge == nil {
		return event.CreateMessage(messageBuilder.
			SetContent("There is no purge being set up.").
			Build())
	}
	if purge.StartID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the start message first.").
			Build())
	}
	if purge.EndID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the end message first.").
			Build())
	}
	if data.TargetID() == purge.StartID {
		return event.CreateMessage(messageBuilder.
			SetContent("You cannot exclude the start message.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if data.TargetID() == purge.EndID {
		return event.CreateMessage(messageBuilder.
			SetContent("You cannot exclude the end message.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	if (!purge.Forwards && (data.TargetID() > purge.StartID || data.TargetID() < purge.EndID)) || (purge.Forwards && (data.TargetID() < purge.StartID || data.TargetID() > purge.EndID)) {
		return event.CreateMessage(messageBuilder.
			SetContent("Message is out of the specified range.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}

	jumpURL := data.TargetMessage().JumpURL()
	if ok := h.controller.ExcludeMessage(purge, data.TargetID()); !ok {
		return event.CreateMessage(messageBuilder.
			SetContentf("[This message](%s) is already excluded.", jumpURL).
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Alright, [this message](%s) has been excluded.", jumpURL).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleInclude(data discord.MessageCommandInteractionData, event *handler.CommandEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	purge := h.controller.Purge(event.Channel().ID())
	if purge == nil {
		return event.CreateMessage(messageBuilder.
			SetContent("There is no purge being set up.").
			Build())
	}
	if purge.StartID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the start message first.").
			Build())
	}
	if purge.EndID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the end message first.").
			Build())
	}

	if ok := h.controller.IncludeMessage(purge, data.TargetID()); !ok {
		return event.CreateMessage(messageBuilder.
			SetContent("Messages have to be excluded to include them back.").
			AddActionRow(
				discord.NewPrimaryButton("Run purge", "/purge/run"),
				discord.NewDangerButton("Cancel purge", "/purge/cancel")).
			Build())
	}
	return event.CreateMessage(messageBuilder.
		SetContentf("Alright, [this message](%s) will not be excluded.", data.TargetMessage().JumpURL()).
		AddActionRow(
			discord.NewPrimaryButton("Run purge", "/purge/run"),
			discord.NewDangerButton("Cancel purge", "/purge/cancel")).
		Build())
}

func (h *Handler) HandleRun(_ discord.ButtonInteractionData, event *handler.ComponentEvent) error {
	messageBuilder := discord.NewMessageCreateBuilder()
	purge := h.controller.Purge(event.Channel().ID())
	if purge == nil {
		return event.CreateMessage(messageBuilder.
			SetContent("There is no purge being set up.").
			Build())
	}
	if purge.StartID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the start message first.").
			Build())
	}
	if purge.EndID == 0 {
		return event.CreateMessage(messageBuilder.
			SetContent("Select the end message first.").
			Build())
	}
	if purge.Running {
		return event.CreateMessage(messageBuilder.
			SetContent("Your purge is already running.").
			Build())
	}
	purge.Running = true
	go func() {
		client := event.Client().Rest()
		page := client.GetMessagesPage(event.Channel().ID(), purge.StartID, purge.BulkLimit)
		pageFunc := page.Previous
		if purge.Forwards {
			pageFunc = page.Next
		}
		excluded := purge.Excluded()
		var total int
		for i := 0; ; i++ {
			if !pageFunc() {
				if !errors.Is(page.Err, rest.ErrNoMorePages) {
					_, err := event.CreateFollowupMessage(messageBuilder.
						SetContentf("There was an error while purging: **%s**.", page.Err.Error()).
						Build())
					if err != nil {
						slog.Error("error while responding with a purge error", tint.Err(err))
					}
					slog.Error("error while running a purge", slog.Any("channel.id", event.Channel().ID()), tint.Err(page.Err))
				}
				return
			}
			messageIDs := make([]snowflake.ID, 0, len(page.Items))
			for _, message := range page.Items {
				if (purge.Forwards && message.ID > purge.EndID) || (!purge.Forwards && message.ID < purge.EndID) { // ignore if fetched but over the end message
					continue
				}
				if !slices.Contains(excluded, message.ID) {
					messageIDs = append(messageIDs, message.ID)
				}
			}
			if err := client.BulkDeleteMessages(event.Channel().ID(), messageIDs); err != nil {
				if _, err := event.CreateFollowupMessage(messageBuilder.
					SetContentf("There was an error while trying to bulk delete: **%s**", err.Error()).
					Build()); err != nil {
					slog.Error("error while responding with a bulk delete error", tint.Err(err))
				}
				slog.Error("error while running a bulk delete", slog.Any("channel.id", event.Channel().ID()), tint.Err(err))
				return
			}
			total += len(messageIDs)
			_, err := event.CreateFollowupMessage(messageBuilder.
				SetContentf("Purged bulk **%d**.. (messages purged so far: **%d**)", i+1, total).
				Build())
			if err != nil {
				slog.Error("error while responding with a purge bulk update", tint.Err(err))
				return
			}

			if slices.ContainsFunc(page.Items, endMessageFunc(purge.EndID)) {
				_, err := event.CreateFollowupMessage(messageBuilder.
					SetContentf("All messages have been purged. Total count: **%d**", total).
					Build())
				if err != nil {
					slog.Error("error while responding with a purge end update", tint.Err(err))
				}
				h.controller.RemovePurge(event.Channel().ID())
				return
			}
		}
	}()
	return event.CreateMessage(messageBuilder.
		SetContent("Running purge..").
		Build())
}
