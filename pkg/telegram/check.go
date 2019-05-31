package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

func (b *Bot) check(msg *tgbotapi.Message) {
	logger := log.WithField("user_id", msg.From.ID)
	logger.Infof("Got %d new members", len(msg.NewChatMembers))

	for _, u := range msg.NewChatMembers {
		check, ok := b.spamList.CheckUser(u.ID)
		if check {
			b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf("User %s is blocked!", u.UserName))
			if !ok {
				b.spamList.Add(u.ID)
			}
		} else {
			b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf("User %s is not blocked! :)", u.UserName))
		}
	}
}
