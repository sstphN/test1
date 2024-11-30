package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testkursa/internal/privateapi"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var userSelections = make(map[int64]UserSelection)

type UserSelection struct {
	Action    string
	Timeframe string
}

func main() {
	// Замените "your_bot4_token_here" на реальный токен бота Bot4
	botToken := os.Getenv("BOT4_TOKEN")
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

	log.Println("Бот успешно запущен и ожидает сообщений")

	for update := range updates {
		if update.Message != nil {
			log.Printf("Получено сообщение от пользователя %d: %s", update.Message.Chat.ID, update.Message.Text)
			HandleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			log.Printf("Получен CallbackQuery от пользователя %d: %s", update.CallbackQuery.From.ID, update.CallbackQuery.Data)
			// Обработка CallbackQuery, если необходимо
		} else {
			log.Printf("Получено неизвестное обновление: %+v", update)
		}
	}
}

func HandleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	text := message.Text

	log.Printf("Обработка сообщения от пользователя %d: %s", chatID, text)

	if strings.HasPrefix(text, "/start") {
		// Парсим параметры из команды /start
		params := strings.SplitN(text, " ", 2)
		if len(params) > 1 {
			selectionParam := params[1]
			parts := strings.SplitN(selectionParam, "_", 2)
			if len(parts) == 2 {
				userSelections[chatID] = UserSelection{
					Action:    parts[0],
					Timeframe: parts[1],
				}
				log.Printf("Получены параметры: Action=%s, Timeframe=%s от пользователя %d", parts[0], parts[1], chatID)
			} else {
				msg := tgbotapi.NewMessage(chatID, "Некорректные параметры. Пожалуйста, вернитесь в основного бота и выберите данные заново.")
				if _, err := bot.Send(msg); err != nil {
					log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
				}
				return
			}
		} else {
			msg := tgbotapi.NewMessage(chatID, "Параметры не переданы. Пожалуйста, вернитесь в основного бота и выберите данные заново.")
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
			}
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Бот начнёт отправлять вам данные.")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
		}

		// Запускаем отправку данных
		go sendDataPeriodically(bot, chatID)
	} else if text == "/stop" {
		// Останавливаем отправку данных
		delete(userSelections, chatID)
		msg := tgbotapi.NewMessage(chatID, "Отправка данных остановлена.")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
		}
	} else {
		msg := tgbotapi.NewMessage(chatID, "Бот отправляет вам данные. Вы можете отправить /stop для остановки.")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
		}
	}
}

func sendDataPeriodically(bot *tgbotapi.BotAPI, chatID int64) {
	ticker := time.NewTicker(1 * time.Minute) // Интервал обновления данных (1 минута для тестирования)
	defer ticker.Stop()

	for {
		// Проверяем, что пользователь всё ещё хочет получать данные
		if _, exists := userSelections[chatID]; !exists {
			log.Printf("Пользователь %d остановил получение данных", chatID)
			return
		}

		sendData(bot, chatID)

		<-ticker.C
	}
}

func sendData(bot *tgbotapi.BotAPI, chatID int64) {
	selection, exists := userSelections[chatID]
	if !exists {
		log.Printf("Нет сохранённых параметров для пользователя %d", chatID)
		return
	}

	var messageText string

	switch selection.Action {
	case "action_gainers":
		topGainers, err := privateapi.FetchTopGainers("futures", selection.Timeframe)
		if err != nil {
			log.Println("Ошибка при получении топ-гейнеров:", err)
			messageText = "Произошла ошибка при получении данных."
		} else {
			messageText = FormatGainersMessage(topGainers, "futures", selection.Timeframe)
		}
	case "action_pumpdump":
		pumpDumpTickers, err := privateapi.FetchPumpDump("futures", selection.Timeframe)
		if err != nil {
			log.Println("Ошибка при получении данных (pumpdump):", err)
			messageText = "Произошла ошибка при получении данных."
		} else {
			messageText = FormatPumpDumpMessage(pumpDumpTickers, "futures", selection.Timeframe)
		}
	default:
		messageText = "Неизвестное действие."
	}

	log.Printf("Отправка данных пользователю %d", chatID)
	msg := tgbotapi.NewMessage(chatID, messageText)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("Ошибка при отправке сообщения пользователю %d: %v", chatID, err)
	}
}

func FormatGainersMessage(tickers []privateapi.Ticker, market, timeframe string) string {
	message := fmt.Sprintf("Топ-гейнеры на рынке %s за %s:\n", market, timeframe)
	for _, ticker := range tickers {
		message += fmt.Sprintf("%s: %s%%\n", ticker.Symbol, ticker.PriceChangePercent)
	}
	return message
}

func FormatPumpDumpMessage(tickers []privateapi.Ticker, market, timeframe string) string {
	message := ""
	for _, ticker := range tickers {
		// Определяем направление движения
		changePercent, _ := strconv.ParseFloat(ticker.PriceChangePercent, 64)
		directionEmoji := "🔴"
		if changePercent > 0 {
			directionEmoji = "🟢"
		}

		// Объём
		formattedVolume := formatVolume(ticker.Volume)

		// Данные о Max Dump и Max Pump за 24 часа
		message += fmt.Sprintf("[❕ %s %s %s%% / 9.9s\n24H Vol: %s\nMax Dump: -%s%% Max Pump: +%s%%]\n\n",
			directionEmoji, ticker.Symbol, ticker.PriceChangePercent, formattedVolume, ticker.MaxDump, ticker.MaxPump)
	}
	return message
}

func formatVolume(volumeStr string) string {
	volume, _ := strconv.ParseFloat(volumeStr, 64)
	if volume >= 1e9 {
		return fmt.Sprintf("%.3f B", volume/1e9)
	} else if volume >= 1e6 {
		return fmt.Sprintf("%.3f M", volume/1e6)
	} else if volume >= 1e3 {
		return fmt.Sprintf("%.3f K", volume/1e3)
	}
	return fmt.Sprintf("%.3f", volume)
}
