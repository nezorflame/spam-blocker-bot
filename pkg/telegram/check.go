package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func (b *Bot) check(msg *tgbotapi.Message) {
	logger := log.WithField("chat_id", msg.Chat.ID)
	logger.Debugf("Got %d new member(s)", len(*msg.NewChatMembers))

	for _, u := range *msg.NewChatMembers {
		check, ok := b.spamList.CheckUser(u.ID)
		if check {
			logger = logger.WithField("user_id", u.ID).WithField("user_name", u.UserName)
			logger.Info("Banning user")
			b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(b.cfg.GetString("messages.blocked"), u.UserName))
			if err := b.ban(msg.Chat.ID, u.ID); err != nil {
				logger.WithError(err).Error("Unable to ban user")
			}
			// add to spamlist if user is not there yet
			if !ok {
				b.spamList.Add(u.ID)
			}
		}
	}
}

func (b *Bot) ban(chatID int64, userID int) error {
	// first ban the user
	allow := false
	resp, err := b.api.RestrictChatMember(tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		CanSendMessages:       &allow,
		CanSendMediaMessages:  &allow,
		CanSendOtherMessages:  &allow,
		CanAddWebPagePreviews: &allow,
	})
	if err != nil {
		return errors.Wrap(err, "unable to ban user")
	}
	if !resp.Ok {
		return errors.Errorf("unable to ban user: %s", resp.Description)
	}

	// then kick the user
	resp, err = b.api.KickChatMember(tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
	})
	if err != nil {
		return errors.Wrap(err, "unable to ban user")
	}
	if !resp.Ok {
		return errors.Errorf("unable to ban user: %s", resp.Description)
	}

	return nil
}
