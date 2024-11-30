package main

import (
	"log"

	"testkursa/internal/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {

	botToken := "7598739623:AAGaqFAXrhQy1urrkRtRx01CvBY9Lwh_pIc"
	if botToken == "" {
		log.Fatal("Bot token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Авторизован под аккаунтом %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			handlers.HandleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			handlers.HandleCallback(bot, update.CallbackQuery)
		}
	}
}
