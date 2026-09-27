package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

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

// gregorianToJalali converts a Gregorian date to the Jalali (Persian) calendar.
// Algorithm based on the widely used jalaali algorithm.
func gregorianToJalali(gy, gm, gd int) (jy, jm, jd int) {
	gy -= 1600
	gm--
	gd--

	gDayNo := 365*gy + (gy+3)/4 - (gy+99)/100 + (gy+399)/400
	gDaysInMonth := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := 0; i < gm; i++ {
		gDayNo += gDaysInMonth[i]
	}
	if gm > 1 && ((gy+1600)%4 == 0 && (gy+1600)%100 != 0 || (gy+1600)%400 == 0) {
		gDayNo++
	}
	gDayNo += gd

	jDayNo := gDayNo - 79

	jNp := jDayNo / 12053
	jDayNo %= 12053

	jy = 979 + 33*jNp + 4*(jDayNo/1461)
	jDayNo %= 1461

	if jDayNo >= 366 {
		jy += (jDayNo - 1) / 365
		jDayNo = (jDayNo - 1) % 365
	}

	if jDayNo < 186 {
		jm = 1 + jDayNo/31
		jd = 1 + jDayNo%31
	} else {
		jm = 7 + (jDayNo-186)/30
		jd = 1 + (jDayNo-186)%30
	}
	return
}

// toPersianDigits converts ASCII digits in a string to Persian digits.
func toPersianDigits(s string) string {
	digits := map[rune]rune{
		'0': '۰', '1': '۱', '2': '۲', '3': '۳', '4': '۴',
		'5': '۵', '6': '۶', '7': '۷', '8': '۸', '9': '۹',
	}
	return strings.Map(func(r rune) rune {
		if d, ok := digits[r]; ok {
			return d
		}
		return r
	}, s)
}

// formatJalali formats a time in the Asia/Tehran timezone as a Jalali date,
// optionally including the clock time. Digits are Persian.
func formatJalali(t time.Time, withTime bool) string {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		loc = time.Local
	}
	t = t.In(loc)
	jy, jm, jd := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	if withTime {
		return toPersianDigits(fmt.Sprintf("%d/%02d/%02d %02d:%02d", jy, jm, jd, t.Hour(), t.Minute()))
	}
	return toPersianDigits(fmt.Sprintf("%d/%02d/%02d", jy, jm, jd))
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
				// inboundsList, _ := suiClient.GetInbounds()
				// inboundMap := make(map[int]sui.Inbound)
				// for _, ib := range inboundsList {
				// 	inboundMap[ib.ID] = ib
				// }

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
						createdStr = formatJalali(time.Unix(targetClient.CreatedAt, 0), true)
					}

					expiryStr := "نامشخص"
					daysLeftStr := ""
					if targetClient.Expiry > 0 {
						expiryTime := time.Unix(targetClient.Expiry, 0)
						expiryStr = formatJalali(expiryTime, false)
						daysLeft := int(time.Until(expiryTime).Hours() / 24)
						if daysLeft < 0 {
							daysLeftStr = fmt.Sprintf(" (منقضی شده)")
						} else {
							daysLeftStr = toPersianDigits(fmt.Sprintf(" (%d روز باقی‌مانده)", daysLeft))
						}
					}

					onlineStr := "آفلاین"
					if targetClient.OnlineAt > 0 {
						onlineStr = formatJalali(time.Unix(targetClient.OnlineAt, 0), true)
					}

					subBase := fmt.Sprintf("%s/%s", subURL, targetClient.Name)

					volumeGB := float64(targetClient.Volume) / (1024 * 1024 * 1024)
					uploadGB := float64(targetClient.TotalUpload) / (1024 * 1024 * 1024)
					downloadGB := float64(targetClient.TotalDownload) / (1024 * 1024 * 1024)
					usedGB := uploadGB + downloadGB
					leftGB := volumeGB - usedGB
					if leftGB < 0 {
						leftGB = 0
					}

					msgText := fmt.Sprintf("👤 اطلاعات کاربر: *%s*\n"+
						"📊 وضعیت: %s\n"+
						"💾 حجم کل: %s GB\n"+
						"⬆️ مصرف آپلود: %s GB\n"+
						"⬇️ مصرف دانلود: %s GB\n"+
						"📥 حجم مصرف‌شده: %s GB\n"+
						"📤 حجم باقی‌مانده: %s GB\n"+
						"📅 تاریخ ایجاد: %s\n"+
						"🗓️ تاریخ پایان: %s%s\n"+
						"🕒 آخرین اتصال: %s\n\n"+
						"🔗 لینک‌های سابسکریپشن:\n"+
						"۱. ساب عمومی (لینک‌ها - v2rayN/v2rayNG):\n`%s`\n"+
						"۲. ساب سینگ‌باکس (JSON - sing-box/Hiddify):\n`%s?format=json`\n"+
						"۳. ساب کلش (Clash - Clash.Meta/Mihomo):\n`%s?format=clash`",
						targetClient.Name,
						status,
						toPersianDigits(fmt.Sprintf("%.2f", volumeGB)),
						toPersianDigits(fmt.Sprintf("%.2f", uploadGB)),
						toPersianDigits(fmt.Sprintf("%.2f", downloadGB)),
						toPersianDigits(fmt.Sprintf("%.2f", usedGB)),
						toPersianDigits(fmt.Sprintf("%.2f", leftGB)),
						createdStr,
						expiryStr, daysLeftStr,
						onlineStr,
						subBase,
						subBase,
						subBase)

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
