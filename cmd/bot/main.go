package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/user/telegram-sui-bot/internal/sui"
)

func parseAdminIDs(str string) map[int64]bool {
	admins := make(map[int64]bool)
	for _, idStr := range strings.Split(str, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err == nil {
			admins[id] = true
		}
	}
	return admins
}

func isAdmin(id int64, admins map[int64]bool) bool {
	return admins[id]
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	suiURL := os.Getenv("SUI_URL")
	suiToken := os.Getenv("SUI_TOKEN")
	adminIDsStr := os.Getenv("ADMIN_IDS")

	if telegramToken == "" || suiURL == "" || suiToken == "" || adminIDsStr == "" {
		log.Fatal("TELEGRAM_TOKEN, SUI_URL, SUI_TOKEN, and ADMIN_IDS must be set")
	}

	adminIDs := parseAdminIDs(adminIDsStr)
	if len(adminIDs) == 0 {
		log.Fatal("ADMIN_IDS must contain at least one valid numeric ID")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	suiClient := sui.NewClient(suiURL, suiToken)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !isAdmin(update.Message.From.ID, adminIDs) {
			bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Unauthorized: You are not an admin."))
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "list_clients":
				clients, err := suiClient.GetClients()
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Error: %v", err)))
					continue
				}

				msg := "Active Clients:\n"
				for _, client := range clients {
					msg += fmt.Sprintf("- %s (Up: %d, Down: %d)\n", client.Name, client.TotalUpload, client.TotalDownload)
				}
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))
			}
		}
	}
}
