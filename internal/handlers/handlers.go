package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var timeframeOptions = []string{"5m", "15m", "1h", "24h"}
var additionalBots = map[string]string{
	"Bot1": "https://t.me/tgtestoviy1_bot",
	"Bot2": "https://t.me/tgtestoviy2_bot",
	"Bot3": "https://t.me/tgtestoviy3_bot",
	"Bot4": "https://t.me/tgtestoviy4_bot",
}

func HandleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Text {
	case "/start":
		sendMainMenu(bot, message.Chat.ID)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Пожалуйста, используйте кнопки для взаимодействия с ботом.")
		bot.Send(msg)
	}
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Gainers", "action_gainers"),
			tgbotapi.NewInlineKeyboardButtonData("Pump/Dump", "action_pumpdump"),
		),
	)
	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Пожалуйста, выберите опцию:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func HandleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	data := callback.Data
	chatID := callback.Message.Chat.ID

	switch {
	case data == "action_gainers" || data == "action_pumpdump":
		sendTimeframeMenu(bot, chatID, data)
	case strings.HasPrefix(data, "timeframe_"):
		parts := strings.Split(data, "|")
		action := parts[1]
		timeframe := parts[2]
		sendBotSelectionMenu(bot, chatID, action, timeframe)
	case strings.HasPrefix(data, "botselect_"):
		parts := strings.Split(data, "|")
		action := parts[1]
		timeframe := parts[2]
		botName := parts[3]
		handleFinalAction(bot, chatID, action, timeframe, botName)
	case data == "back_to_main":
		sendMainMenu(bot, chatID)
	}

	// Подтверждаем callback, чтобы Telegram не показывал часы ожидания
	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func sendTimeframeMenu(bot *tgbotapi.BotAPI, chatID int64, action string) {
	var buttons [][]tgbotapi.InlineKeyboardButton

	for _, tf := range timeframeOptions {
		callbackData := fmt.Sprintf("timeframe_|%s|%s", action, tf)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(tf, callbackData),
		))
	}

	// Кнопка "Назад"
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Назад", "back_to_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите таймфрейм:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func sendBotSelectionMenu(bot *tgbotapi.BotAPI, chatID int64, action, timeframe string) {
	var buttons [][]tgbotapi.InlineKeyboardButton

	for botName := range additionalBots {
		callbackData := fmt.Sprintf("botselect_|%s|%s|%s", action, timeframe, botName)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(botName, callbackData),
		))
	}

	// Кнопка "Назад"
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Назад", "back_to_main"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	msg := tgbotapi.NewMessage(chatID, "Пожалуйста, выберите бота для получения данных:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleFinalAction(bot *tgbotapi.BotAPI, chatID int64, action, timeframe, botName string) {
	botLink, exists := additionalBots[botName]
	if !exists {
		msg := tgbotapi.NewMessage(chatID, "Выбранный бот не найден.")
		bot.Send(msg)
		return
	}

	// Используем кнопку с прямой ссылкой на бота
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(fmt.Sprintf("Перейти в %s", botName), botLink),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Пожалуйста, нажмите кнопку ниже, чтобы перейти в бота:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}
