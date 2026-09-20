package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

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

type SessionData struct {
	Name     string
	Inbounds []int
	Volume   int
	Step     string
}

var sessions = make(map[int64]*SessionData)

func isAdmin(id int64, admins map[int64]bool) bool {
	return admins[id]
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64, messageID *int) {
	msg := tgbotapi.NewMessage(chatID, "👋 خوش آمدید! یکی از گزینه‌ها را انتخاب کنید:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 لیست کاربران", "list_clients"),
			tgbotapi.NewInlineKeyboardButtonData("➕ ایجاد کاربر", "create_client_flow"),
		),
	)
	if messageID != nil {
		bot.Send(tgbotapi.NewEditMessageText(chatID, *messageID, msg.Text))
		bot.Send(tgbotapi.NewEditMessageReplyMarkup(chatID, *messageID, msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)))
	} else {
		bot.Send(msg)
	}
}

func handleListClients(bot *tgbotapi.BotAPI, chatID int64, suiClient *sui.Client) {
	clients, err := suiClient.GetClients()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا: %v", err)))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, client := range clients {
		btnText := client.Name
		if !client.Enable {
			btnText = "🔴 " + btnText
		} else {
			btnText = "🟢 " + btnText
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("client_info_%d", client.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به منو اصلی", "main_menu")))

	msg := tgbotapi.NewMessage(chatID, "📋 لیست کاربران (برای جزئیات کلیک کنید):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(msg)
}

func handleCreateClientFlow(bot *tgbotapi.BotAPI, chatID int64, userID int64, suiClient *sui.Client) {
	inbounds, err := suiClient.GetInbounds()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا در دریافت اینباندها: %v", err)))
		return
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, inbound := range inbounds {
		displayName := inbound.Remark
		if displayName == "" {
			displayName = inbound.Tag
		}
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(displayName, fmt.Sprintf("inbound_%d", inbound.ID)),
		)
		keyboard = append(keyboard, row)
	}
	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ تایید انتخاب", "confirm_selection"),
	))
	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به منو اصلی", "main_menu")))

	sessions[userID] = &SessionData{Step: "SELECT_INBOUNDS"}
	msg := tgbotapi.NewMessage(chatID, "⚙️ انتخاب اینباندها (برای افزودن/حذف کلیک کنید):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	bot.Send(msg)
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	suiURL := os.Getenv("SUI_URL")
	suiToken := os.Getenv("SUI_TOKEN")
	adminIDsStr := os.Getenv("ADMIN_IDS")

	if telegramToken == "" || suiURL == "" || suiToken == "" || adminIDsStr == "" {
		log.Fatal("TELEGRAM_TOKEN, SUI_URL, SUI_TOKEN, and ADMIN_IDS must be set")
	}

	// Try fetching subURL from panel settings
	adminIDs := parseAdminIDs(adminIDsStr)
	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	suiClient := sui.NewClient(suiURL, suiToken)

	settings, err := suiClient.GetSettings()
	subURL := ""
	if err == nil {
		webDomain, _ := settings["webDomain"].(string)
		subPort, _ := settings["subPort"]
		subPath, _ := settings["subPath"].(string)

		if webDomain != "" && subPort != nil && subPath != "" {
			var portStr string
			switch v := subPort.(type) {
			case float64:
				portStr = fmt.Sprintf("%.0f", v)
			case int:
				portStr = fmt.Sprintf("%d", v)
			case string:
				portStr = v
			}
			subURL = fmt.Sprintf("https://%s:%s%s", webDomain, portStr, subPath)
		}
	}
	if subURL == "" {
		subURL = os.Getenv("SUB_URL") // Fallback
		if subURL == "" {
			subURL = suiURL // Fallback
		}
	}
	subURL = strings.TrimSuffix(subURL, "/")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			query := update.CallbackQuery
			userID := query.From.ID
			chatID := query.Message.Chat.ID
			messageID := query.Message.MessageID

			if !isAdmin(userID, adminIDs) {
				bot.Send(tgbotapi.NewMessage(chatID, "🚫 غیرمجاز."))
				continue
			}

			session := sessions[userID]

			if query.Data == "list_clients" {
				bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
				handleListClients(bot, chatID, suiClient)
			} else if query.Data == "create_client_flow" {
				bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
				handleCreateClientFlow(bot, chatID, userID, suiClient)
			} else if query.Data == "main_menu" {
				bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
				sendMainMenu(bot, chatID, nil)
			} else if strings.HasPrefix(query.Data, "client_info_") {
				clientID, _ := strconv.Atoi(strings.TrimPrefix(query.Data, "client_info_"))
				clients, _ := suiClient.GetClients()
				inboundsList, _ := suiClient.GetInbounds()
				inboundMap := make(map[int]sui.Inbound)
				for _, ib := range inboundsList {
					inboundMap[ib.ID] = ib
				}

				var targetClient *sui.ClientInfo
				for _, c := range clients {
					if c.ID == clientID {
						targetClient = &c
						break
					}
				}
				if targetClient != nil {
					status := "🟢 فعال"
					if !targetClient.Enable {
						status = "🔴 غیرفعال"
					}

					createdStr := "نامشخص"
					if targetClient.CreatedAt > 0 {
						createdStr = time.Unix(targetClient.CreatedAt, 0).Format("2006-01-02 15:04")
					}

					onlineStr := "آفلاین"
					if targetClient.OnlineAt > 0 {
						onlineStr = time.Unix(targetClient.OnlineAt, 0).Format("2006-01-02 15:04")
					}

					subBase := fmt.Sprintf("%s/%s", subURL, targetClient.Name)

					// Build individual config links for this user's inbounds
					configLinks := ""
					for _, ibID := range targetClient.Inbounds {
						ib, ok := inboundMap[ibID]
						if ok {
							configLinks += fmt.Sprintf("• %s (%s):\n`%s/%s/%s`\n", ib.Remark, ib.Type, subURL, ib.Remark, targetClient.Name)
						}
					}

					msgText := fmt.Sprintf("👤 اطلاعات کاربر: *%s*\n"+
						"📊 وضعیت: %s\n"+
						"💾 حجم کل: %.2f GB\n"+
						"⬆️ مصرف آپلود: %.2f GB\n"+
						"⬇️ مصرف دانلود: %.2f GB\n"+
						"📅 تاریخ ایجاد: %s\n"+
						"🕒 آخرین اتصال: %s\n\n"+
						"🔗 لینک‌های سابسکریپشن:\n"+
						"۱. ساب عمومی (JSON):\n`%s`\n"+
						"۲. ساب کلش (Clash):\n`%s?clash=1`\n"+
						"۳. ساب سینگ‌باکس (Sing-box):\n`%s?singbox=1`\n\n"+
						"🔗 لینک‌های کانفیگ تکی:\n%s",
						targetClient.Name,
						status,
						float64(targetClient.Volume)/(1024*1024*1024),
						float64(targetClient.TotalUpload)/(1024*1024*1024),
						float64(targetClient.TotalDownload)/(1024*1024*1024),
						createdStr,
						onlineStr,
						subBase,
						subBase,
						subBase,
						configLinks)

					msg := tgbotapi.NewMessage(chatID, msgText)
					msg.ParseMode = "Markdown"
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به لیست", "list_clients"),
						tgbotapi.NewInlineKeyboardButtonData("🏠 منو اصلی", "main_menu"),
					))

					bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
					bot.Send(msg)
				}
			} else if strings.HasPrefix(query.Data, "inbound_") {
				inboundID, _ := strconv.Atoi(strings.TrimPrefix(query.Data, "inbound_"))
				found := false
				for i, id := range session.Inbounds {
					if id == inboundID {
						session.Inbounds = append(session.Inbounds[:i], session.Inbounds[i+1:]...)
						found = true
						break
					}
				}
				if !found {
					session.Inbounds = append(session.Inbounds, inboundID)
				}
				bot.Request(tgbotapi.NewCallback(query.ID, fmt.Sprintf("✅ انتخاب شد: %v", session.Inbounds)))
			} else if query.Data == "confirm_selection" {
				session.Step = "INPUT_NAME"
				bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
				bot.Send(tgbotapi.NewMessage(chatID, "👤 نام کاربر را وارد کنید:"))
			} else if query.Data == "confirm_create" {
				client, err := suiClient.CreateClient(session.Name, session.Inbounds, session.Volume)
				bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا در ایجاد کاربر: %v", err)))
				} else {
					bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ کاربر %s با موفقیت ایجاد شد!\n(۳۰ روزه)", client.Name)))
				}
				delete(sessions, userID)
				sendMainMenu(bot, chatID, nil)
			}
			continue
		}

		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID

		if !isAdmin(userID, adminIDs) {
			continue
		}

		session := sessions[userID]
		if session != nil {
			switch session.Step {
			case "INPUT_NAME":
				session.Name = update.Message.Text
				session.Step = "INPUT_VOLUME"
				bot.Send(tgbotapi.NewMessage(chatID, "💾 حجم کاربر را به گیگابایت وارد کنید:"))
			case "INPUT_VOLUME":
				vol, _ := strconv.Atoi(update.Message.Text)
				session.Volume = vol
				session.Step = "CONFIRM"
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📊 تایید اطلاعات:\nنام: %s\nحجم: %d GB\nاینباندها: %v\nمدت: ۳۰ روز", session.Name, session.Volume, session.Inbounds))
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✅ تایید نهایی", "confirm_create")))
				bot.Send(msg)
			}
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				sendMainMenu(bot, chatID, nil)
			}
		}
	}
}
