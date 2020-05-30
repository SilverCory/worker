package listeners

import (
	"fmt"
	"github.com/TicketsBot/common/eventforwarding"
	"github.com/TicketsBot/common/premium"
	"github.com/TicketsBot/database"
	"github.com/TicketsBot/worker"
	"github.com/TicketsBot/worker/bot/command"
	"github.com/TicketsBot/worker/bot/dbclient"
	"github.com/TicketsBot/worker/bot/modmail/logic"
	modmailutils "github.com/TicketsBot/worker/bot/modmail/utils"
	"github.com/TicketsBot/worker/bot/sentry"
	"github.com/TicketsBot/worker/bot/utils"
	"github.com/rxdn/gdl/gateway/payloads/events"
	"github.com/rxdn/gdl/rest"
	"github.com/rxdn/gdl/rest/request"
	"strconv"
	"strings"
)

func OnDirectMessage(worker *worker.Context, e *events.MessageCreate, extra eventforwarding.Extra) {
	if e.Author.Bot {
		return
	}

	if e.GuildId != 0 { // DMs only
		return
	}

	ctx := command.CommandContext{
		Worker:      worker,
		Message:     e.Message,
		ShouldReact: true,
		IsFromPanel: false,
	}

	session, err := dbclient.Client.ModmailSession.GetByUser(worker.BotId, e.Author.Id)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
		return
	}

	// Create DM channel
	dmChannel, err := worker.CreateDM(e.Author.Id)
	if err != nil {
		sentry.LogWithContext(err, ctx.ToErrorContext()) // User probably has DMs disabled
		return
	}

	// No active session
	if session.UserId == 0 {
		guilds := modmailutils.GetMutualGuilds(ctx.Worker, ctx.Author.Id)

		if len(e.Message.Content) == 0 {
			modmailutils.SendModMailIntro(ctx, dmChannel.Id)
			return
		}

		split := strings.Split(e.Message.Content, " ")

		targetGuildNumber, err := strconv.Atoi(split[0])
		if err != nil || targetGuildNumber < 1 || targetGuildNumber > len(guilds) {
			modmailutils.SendModMailIntro(ctx, dmChannel.Id)
			return
		}

		targetGuild := guilds[targetGuildNumber-1]

		// Check blacklist
		isBlacklisted, err := dbclient.Client.Blacklist.IsBlacklisted(targetGuild.Id, ctx.Author.Id)
		if err != nil {
			sentry.ErrorWithContext(err, ctx.ToErrorContext())
		}

		if isBlacklisted {
			utils.SendEmbed(worker, dmChannel.Id, utils.Red, "Error", "You are blacklisted in this server!", nil, 30, true)
			return
		}

		utils.SendEmbed(worker, dmChannel.Id, utils.Green, "Modmail", fmt.Sprintf("Your modmail ticket in %s has been opened! Use `t!close` to close the session.", targetGuild.Name), nil, 0, true)

		// Send guild's welcome message
		welcomeMessage, err := dbclient.Client.WelcomeMessages.Get(targetGuild.Id)
		if err != nil {
			sentry.ErrorWithContext(err, ctx.ToErrorContext())
			welcomeMessage = "Thank you for contacting support.\nPlease describe your issue (and provide an invite to your server if applicable) and wait for a response."
		}

		welcomeMessageId, err := utils.SendEmbedWithResponse(worker, dmChannel.Id, utils.Green, "Modmail", welcomeMessage, nil, 0, true)
		if err != nil {
			utils.SendEmbed(worker, dmChannel.Id, utils.Red, "Error", fmt.Sprintf("An error has occurred: %s", err.Error()), nil, 30, true)
			return
		}

		staffChannel, err := logic.OpenModMailTicket(worker, targetGuild, e.Author, welcomeMessageId.Id)
		if err != nil {
			utils.SendEmbed(worker, dmChannel.Id, utils.Red, "Error", fmt.Sprintf("An error has occurred: %s", err.Error()), nil, 30, true)
			return
		}

		utils.SendEmbed(worker, staffChannel, utils.Green, "Modmail", welcomeMessage, nil, 0, true)
	} else { // Forward message to guild or handle command
		// Update context
		ctx.PremiumTier = utils.PremiumClient.GetTierByGuildId(session.GuildId, true, worker.Token, worker.RateLimiter)
		ctx.ChannelId = dmChannel.Id

		// Parse DM channel ID
		ctx.ChannelId = dmChannel.Id

		var isCommand bool
		ctx, isCommand = handleCommand(ctx, session)

		if isCommand {
			switch ctx.Root {
			case "close":
				logic.HandleClose(session, ctx)
			}
		} else {
			sendMessage(session, ctx, dmChannel.Id)
		}
	}
}

func sendMessage(session database.ModmailSession, ctx command.CommandContext, dmChannel uint64) {
	// Preferably send via a webhook
	webhook, err := dbclient.Client.ModmailWebhook.Get(session.Uuid)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
	}

	success := false
	if webhook.WebhookId != 0 {
		success = executeWebhook(ctx.Worker, webhook, rest.WebhookBody{
			Content:   ctx.Message.Content,
			Username:  ctx.Message.Author.Username,
			AvatarUrl: ctx.Author.AvatarUrl(256),
		})
	}

	if !success {
		if _, err := ctx.Worker.CreateMessage(session.StaffChannelId, ctx.Message.Content); err != nil {
			utils.SendEmbed(ctx.Worker, dmChannel, utils.Red, "Error", fmt.Sprintf("An error has occurred: `%s`", err.Error()), nil, 30, ctx.PremiumTier > premium.None)
			sentry.LogWithContext(err, ctx.ToErrorContext())
		}
	}

	// forward attachments
	// don't re-upload attachments incase user has uploaded TOS breaking attachment
	if len(ctx.Message.Attachments) > 0 {
		var content string
		if len(ctx.Message.Attachments) == 1 {
			content = fmt.Sprintf("%s attached a file:", ctx.Author.Mention())
		} else {
			content = fmt.Sprintf("%s attached files:", ctx.Author.Mention())
		}

		for _, attachment := range ctx.Message.Attachments {
			content += fmt.Sprintf("\n▶️ %s", attachment.ProxyUrl)
		}

		if _, err := ctx.Worker.CreateMessage(session.StaffChannelId, content); err != nil {
			utils.SendEmbed(ctx.Worker, dmChannel, utils.Red, "Error", fmt.Sprintf("An error has occurred: `%s`", err.Error()), nil, 30, ctx.PremiumTier > premium.None)
			sentry.LogWithContext(err, ctx.ToErrorContext())
		}
	}
}

func executeWebhook(worker *worker.Context, webhook database.ModmailWebhook, data rest.WebhookBody) bool {
	_, err := worker.ExecuteWebhook(webhook.WebhookId, webhook.WebhookToken, true, data)

	if err == request.ErrForbidden || err == request.ErrNotFound {
		go dbclient.Client.ModmailWebhook.Delete(webhook.Uuid)
		return false
	} else {
		return true
	}
}

// TODO: Make this less hacky
func handleCommand(ctx command.CommandContext, session database.ModmailSession) (command.CommandContext, bool) {
	customPrefix, err := dbclient.Client.Prefix.Get(session.GuildId)
	if err != nil {
		sentry.ErrorWithContext(err, ctx.ToErrorContext())
	}

	var usedPrefix string

	if strings.HasPrefix(ctx.Message.Content, utils.DEFAULT_PREFIX) {
		usedPrefix = utils.DEFAULT_PREFIX
	} else if customPrefix != "" && strings.HasPrefix(ctx.Message.Content, customPrefix) {
		usedPrefix = customPrefix
	} else { // Not a command
		return ctx, false
	}

	split := strings.Split(ctx.Message.Content, " ")
	root := strings.TrimPrefix(split[0], usedPrefix)

	args := make([]string, 0)
	if len(split) > 1 {
		for _, arg := range split[1:] {
			if arg != "" {
				args = append(args, arg)
			}
		}
	}

	ctx.Args = args
	ctx.Root = root

	return ctx, true
}