package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/user/telegram-sui-bot/internal/sui"
	"github.com/user/telegram-sui-bot/internal/packages"
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
	// Data for inbound operations
	InboundID int
	Inbound   map[string]interface{}
	Package   packages.Package
}

// sessions protected by sessionsMu
var (
	sessions  = make(map[int64]*SessionData)
	sessionsMu sync.RWMutex
)

func getSession(userID int64) *SessionData {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	return sessions[userID]
}

func setSession(userID int64, s *SessionData) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	if s == nil {
		delete(sessions, userID)
	} else {
		sessions[userID] = s
	}
}

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

// escapeMarkdown escapes special Markdown characters to prevent injection.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
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

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64, messageID *int, userName string) {
	greeting := "👋 خوش آمدید! یکی از گزینه‌ها را انتخاب کنید:"
	if userName != "" {
		safeName := escapeMarkdown(userName)
		greeting = fmt.Sprintf("سلام %s! خوش آمدید 👋", safeName)
	}
	msg := tgbotapi.NewMessage(chatID, greeting)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 لیست کاربران", "list_clients"),
			tgbotapi.NewInlineKeyboardButtonData("➕ ایجاد کاربر", "create_client_flow"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ مدیریت اینباندها", "list_inbounds"),
				tgbotapi.NewInlineKeyboardButtonData("📦 مدیریت پکیج‌ها", "package_management"),
		),
	)
	if messageID != nil {
		if _, err := bot.Send(tgbotapi.NewEditMessageText(chatID, *messageID, msg.Text)); err != nil {
			log.Printf("EditMessageText error: %v", err)
		}
		if _, err := bot.Send(tgbotapi.NewEditMessageReplyMarkup(chatID, *messageID, msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup))); err != nil {
			log.Printf("EditMessageReplyMarkup error: %v", err)
		}
	} else {
		if _, err := bot.Send(msg); err != nil {
			log.Printf("SendMessage error: %v", err)
		}
	}
}

func sendClientInfo(bot *tgbotapi.BotAPI, chatID int64, messageID int, targetClient *sui.ClientInfo, subURL string) {
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
			daysLeftStr = " (منقضی شده)"
		} else {
			daysLeftStr = toPersianDigits(fmt.Sprintf(" (%d روز باقیمانده)", daysLeft))
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

	// Escape user-controlled fields for Markdown safety
	safeName := escapeMarkdown(targetClient.Name)
	safeSubBase := escapeMarkdown(subBase)

	msgText := fmt.Sprintf("👤 اطلاعات کاربر: *%s*\n"+
		"📊 وضعیت: %s\n"+
		"💾 حجم کل: %s GB\n"+
		"⬆️ مصرف آپلود: %s GB\n"+
		"⬇️ مصرف دانلود: %s GB\n"+
		"📥 حجم مصرف‌شده: %s GB\n"+
		"📤 حجم باقیمانده: %s GB\n"+
		"📅 تاریخ ایجاد: %s\n"+
		"🗓️ تاریخ پایان: %s%s\n"+
		"🕒 آخرین اتصال: %s\n\n"+
		"🔗 *لینک‌های سابسکریپشن:*\n"+
		"(برای کپی کردن، روی لینک‌ها ضربه بزنید)\n\n"+
		"🌐 *ساب عمومی (v2rayN/v2rayNG):*\n`%s`\n\n"+
		"📦 *ساب سینگ‌باکس (JSON):*\n`%s?format=json`\n\n"+
		"⚔️ *ساب کلش (Clash Meta):*\n`%s?format=clash`",
		safeName,
		status,
		toPersianDigits(fmt.Sprintf("%.2f", volumeGB)),
		toPersianDigits(fmt.Sprintf("%.2f", uploadGB)),
		toPersianDigits(fmt.Sprintf("%.2f", downloadGB)),
		toPersianDigits(fmt.Sprintf("%.2f", usedGB)),
		toPersianDigits(fmt.Sprintf("%.2f", leftGB)),
		createdStr,
		expiryStr, daysLeftStr,
		onlineStr,
		safeSubBase,
		safeSubBase,
		safeSubBase)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "Markdown"

	toggleText := "🔴 غیرفعال کردن"
	if !targetClient.Enable {
		toggleText = "🟢 فعال کردن"
	}

	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggleText, fmt.Sprintf("client_toggle_%d_%v", targetClient.ID, targetClient.Enable)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به لیست", "list_clients"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 منو اصلی", "main_menu"),
		),
	)

	if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		log.Printf("DeleteMessage error: %v", err)
	}
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handleListClients(bot *tgbotapi.BotAPI, chatID int64, suiClient *sui.Client) {
	clients, err := suiClient.GetClients()
	if err != nil {
		log.Printf("GetClients error: %v", err)
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
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handleListInbounds(bot *tgbotapi.BotAPI, chatID int64, suiClient *sui.Client) {
	inbounds, err := suiClient.GetInbounds()
	if err != nil {
		log.Printf("GetInbounds error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا در دریافت اینباندها: %v", err)))
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, ib := range inbounds {
		btnText := fmt.Sprintf("%s (%s)", ib.Remark, ib.Type)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("inbound_info_%d", ib.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ افزودن اینباند", "create_inbound_flow")))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به منو اصلی", "main_menu")))

	msg := tgbotapi.NewMessage(chatID, "⚙️ لیست اینباندها:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handlePackageManagement(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📦 مدیریت پکیج‌ها:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ افزودن پکیج", "create_package_flow"),
			tgbotapi.NewInlineKeyboardButtonData("📋 لیست پکیج‌ها", "list_packages"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ ویرایش پکیج", "edit_package_flow"),
			tgbotapi.NewInlineKeyboardButtonData("🗑 حذف پکیج", "remove_package_flow"),
		),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به منو اصلی", "main_menu")),
	)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handleListPackages(bot *tgbotapi.BotAPI, chatID int64) {
	pkgs, err := packages.LoadPackages()
	if err != nil {
		log.Printf("LoadPackages error: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ خطا در بارگذاری پکیج‌ها"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, pkg := range pkgs {
		btnText := fmt.Sprintf("%s (%d GB, %d روز)", pkg.Name, pkg.Volume, pkg.Duration)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(btnText, fmt.Sprintf("package_info_%s", pkg.ID)),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به مدیریت پکیج‌ها", "package_management")))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("🏠 منو اصلی", "main_menu")))

	msg := tgbotapi.NewMessage(chatID, "📦 لیست پکیج‌ها:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handleCreatePackageFlow(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	setSession(userID, &SessionData{Step: "INPUT_PACKAGE_NAME"})
	msg := tgbotapi.NewMessage(chatID, "🏷 نام پکیج را وارد کنید:")
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func handleEditPackageFlow(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	pkgs, err := packages.LoadPackages()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ خطا در بارگذاری پکیج‌ها"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, pkg := range pkgs {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(pkg.Name, fmt.Sprintf("edit_pkg_%s", pkg.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "✏️ پکیجی را برای ویرایش انتخاب کنید:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(msg)
}

func handleRemovePackageFlow(bot *tgbotapi.BotAPI, chatID int64, userID int64) {
	pkgs, err := packages.LoadPackages()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ خطا در بارگذاری پکیج‌ها"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, pkg := range pkgs {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(pkg.Name, fmt.Sprintf("remove_pkg_%s", pkg.ID)),
		))
	}
	msg := tgbotapi.NewMessage(chatID, "🗑 پکیجی را برای حذف انتخاب کنید:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	bot.Send(msg)
}

func handleCreateClientFlow(bot *tgbotapi.BotAPI, chatID int64, userID int64, suiClient *sui.Client) {
	inbounds, err := suiClient.GetInbounds()
	if err != nil {
		log.Printf("GetInbounds error: %v", err)
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

	setSession(userID, &SessionData{Step: "SELECT_INBOUNDS"})
	msg := tgbotapi.NewMessage(chatID, "⚙️ انتخاب اینباندها (برای افزودن/حذف کلیک کنید):")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	if _, err := bot.Send(msg); err != nil {
		log.Printf("SendMessage error: %v", err)
	}
}

func main() {
	// Load .env from current directory or parent
	if err := godotenv.Load(); err != nil {
		// Try parent directory
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("No .env file found")
		}
	}

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	suiURL := os.Getenv("SUI_URL")
	suiToken := os.Getenv("SUI_TOKEN")
	adminIDsStr := os.Getenv("ADMIN_IDS")

	if telegramToken == "" || suiURL == "" || suiToken == "" || adminIDsStr == "" {
		log.Fatal("TELEGRAM_TOKEN, SUI_URL, SUI_TOKEN, and ADMIN_IDS must be set")
	}

	if len(parseAdminIDs(adminIDsStr)) == 0 {
		log.Fatal("ADMIN_IDS must contain at least one valid user ID")
	}

	adminIDs := parseAdminIDs(adminIDsStr)
	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Panic(err)
	}

	suiClient := sui.NewClient(suiURL, suiToken)

	// Try fetching subURL from panel settings
	settings, err := suiClient.GetSettings()
	subURL := ""
	if err == nil {
		webDomain, _ := settings["webDomain"].(string)
		subPortVal, ok := settings["subPort"]
		subPath, _ := settings["subPath"].(string)

		if webDomain != "" && ok && subPath != "" {
			var portStr string
			switch v := subPortVal.(type) {
			case float64:
				portStr = fmt.Sprintf("%.0f", v)
			case int:
				portStr = fmt.Sprintf("%d", v)
			case int64:
				portStr = fmt.Sprintf("%d", v)
			case string:
				portStr = v
			}
			if portStr != "" {
				subURL = fmt.Sprintf("https://%s:%s%s", webDomain, portStr, subPath)
			}
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
			userName := query.From.FirstName

			if !isAdmin(userID, adminIDs) {
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "🚫 غیرمجاز.")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}
				continue
			}

			session := getSession(userID)

			if query.Data == "list_clients" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				handleListClients(bot, chatID, suiClient)
				} else if query.Data == "list_inbounds" {
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					handleListInbounds(bot, chatID, suiClient)
				} else if query.Data == "package_management" {
						if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
							log.Printf("DeleteMessage error: %v", err)
						}
						handlePackageManagement(bot, chatID)
					} else if query.Data == "list_packages" {
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					handleListPackages(bot, chatID)
			} else if query.Data == "create_package_flow" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				handleCreatePackageFlow(bot, chatID, userID)
			} else if query.Data == "edit_package_flow" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				handleEditPackageFlow(bot, chatID, userID)
			} else if query.Data == "remove_package_flow" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				handleRemovePackageFlow(bot, chatID, userID)
			} else if query.Data == "create_client_flow" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				handleCreateClientFlow(bot, chatID, userID, suiClient)
			} else if strings.HasPrefix(query.Data, "edit_pkg_") {
				pkgID := strings.TrimPrefix(query.Data, "edit_pkg_")
				// ... implementation for editing
				bot.Request(tgbotapi.NewCallback(query.ID, fmt.Sprintf("ویرایش پکیج %s", pkgID)))
			} else if strings.HasPrefix(query.Data, "remove_pkg_") {
				pkgID := strings.TrimPrefix(query.Data, "remove_pkg_")
				err := packages.DeletePackage(pkgID)
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ خطا در حذف پکیج"))
				} else {
					bot.Request(tgbotapi.NewCallback(query.ID, "✅ پکیج حذف شد"))
				}
			} else if query.Data == "main_menu" {
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				sendMainMenu(bot, chatID, nil, userName)
			} else if query.Data == "create_inbound_flow" {
				setSession(userID, &SessionData{Step: "INPUT_INBOUND_TAG"})
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "🏷 تگ اینباند را وارد کنید (مثلا vless-reality):")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}
			} else if strings.HasPrefix(query.Data, "client_info_") {
				clientIDStr := strings.TrimPrefix(query.Data, "client_info_")
				clientID, err := strconv.Atoi(clientIDStr)
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ شناسه نامعتبر"))
					continue
				}

				clients, err := suiClient.GetClients()
				if err != nil {
					log.Printf("GetClients error: %v", err)
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ خطا در دریافت کاربران"))
					continue
				}

				var targetClient *sui.ClientInfo
				for _, c := range clients {
					if c.ID == clientID {
						targetClient = &c
						break
					}
				}
				if targetClient != nil {
					sendClientInfo(bot, chatID, messageID, targetClient, subURL)
				}
			} else if strings.HasPrefix(query.Data, "client_toggle_") {
				parts := strings.Split(strings.TrimPrefix(query.Data, "client_toggle_"), "_")
				if len(parts) != 2 {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ فرمت نامعتبر"))
					continue
				}
				clientID, err := strconv.Atoi(parts[0])
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ شناسه نامعتبر"))
					continue
				}
				currentEnable, err := strconv.ParseBool(parts[1])
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ وضعیت نامعتبر"))
					continue
				}

				newEnable := !currentEnable
				log.Printf("Toggling client %d: enable=%v -> %v", clientID, currentEnable, newEnable)
				updatedClient, err := suiClient.UpdateClient(clientID, map[string]interface{}{"enable": newEnable})
				if err != nil {
					log.Printf("UpdateClient error for client %d: %v", clientID, err)
					bot.Request(tgbotapi.NewCallback(query.ID, fmt.Sprintf("❌ خطا: %v", err)))
				} else {
					bot.Request(tgbotapi.NewCallback(query.ID, fmt.Sprintf("✅ کاربر %s شد", map[bool]string{true: "فعال", false: "غیرفعال"}[newEnable])))
					log.Printf("Client %d toggled successfully: %+v", clientID, updatedClient)

					// Display updated client info
					if updatedClient != nil {
						sendClientInfo(bot, chatID, messageID, updatedClient, subURL)
					}
				}
			} else if strings.HasPrefix(query.Data, "inbound_info_") {
				inboundIDStr := strings.TrimPrefix(query.Data, "inbound_info_")
				inboundID, err := strconv.Atoi(inboundIDStr)
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ شناسه نامعتبر"))
					continue
				}

				inbounds, err := suiClient.GetInbounds()
				if err != nil {
					log.Printf("GetInbounds error: %v", err)
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ خطا در دریافت اینباندها"))
					continue
				}

				var targetInbound *sui.Inbound
				for _, ib := range inbounds {
					if ib.ID == inboundID {
						targetInbound = &ib
						break
					}
				}
				if targetInbound != nil {
					msgText := fmt.Sprintf("⚙️ اطلاعات اینباند:\n\n"+
						"🏷 نام: %s\n"+
						"🔖 تگ: %s\n"+
						"🔀 نوع: %s\n\n"+
						"💡 _توجه: این پنل تنظیمات پیشرفته را به صورت مستقیم در ویرایشگرِ وب مدیریت می‌کند._",
						escapeMarkdown(targetInbound.Remark), escapeMarkdown(targetInbound.Tag), escapeMarkdown(targetInbound.Type))

					msg := tgbotapi.NewMessage(chatID, msgText)
					msg.ParseMode = "Markdown"
					msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData("🔙 بازگشت به لیست", "list_inbounds"),
							tgbotapi.NewInlineKeyboardButtonData("🏠 منو اصلی", "main_menu"),
						),
					)
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					if _, err := bot.Send(msg); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
				}
			} else if strings.HasPrefix(query.Data, "inbound_") && !strings.HasPrefix(query.Data, "inbound_info_") {
				if session == nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ سشن منقضی شده"))
					continue
				}
				inboundIDStr := strings.TrimPrefix(query.Data, "inbound_")
				inboundID, err := strconv.Atoi(inboundIDStr)
				if err != nil {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ شناسه نامعتبر"))
					continue
				}

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
				setSession(userID, session) // Save updated session
				bot.Request(tgbotapi.NewCallback(query.ID, fmt.Sprintf("✅ انتخاب شد: %v", session.Inbounds)))
			} else if query.Data == "confirm_selection" {
				if session == nil {
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ سشن منقضی شده. لطفا مجدد تلاش کنید.")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				if len(session.Inbounds) == 0 {
					bot.Request(tgbotapi.NewCallback(query.ID, "❌ حداقل یک اینباند انتخاب کنید"))
					continue
				}
				session.Step = "INPUT_NAME"
				setSession(userID, session)
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "👤 نام کاربر را وارد کنید:")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}
			} else if query.Data == "confirm_create" {
				if session == nil {
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ سشن منقضی شده. لطفا مجدد تلاش کنید.")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				if session.Name == "" || len(session.Inbounds) == 0 || session.Volume <= 0 {
					if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
						log.Printf("DeleteMessage error: %v", err)
					}
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ اطلاعات ناقص است. لطفا مجدد تلاش کنید.")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					setSession(userID, nil)
					sendMainMenu(bot, chatID, nil, "")
					continue
				}

				inboundType := "vmess"
				allInbounds, errIn := suiClient.GetInbounds()
				if errIn == nil && len(session.Inbounds) > 0 {
					for _, in := range allInbounds {
						if in.ID == session.Inbounds[0] {
							inboundType = in.Type
							break
						}
					}
				}
				client, err := suiClient.CreateClient(session.Name, session.Inbounds, session.Volume, 30, inboundType)
				if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
					log.Printf("DeleteMessage error: %v", err)
				}
				if err != nil {
					log.Printf("CreateClient error: %v", err)
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا در ایجاد کاربر: %v", err))); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
				} else {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ کاربر %s با موفقیت ایجاد شد!\n(تاریخ انقضا: %s)", escapeMarkdown(client.Name), time.Unix(client.Expiry, 0).Format("2006-01-02")))); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
				}
				setSession(userID, nil)
				sendMainMenu(bot, chatID, nil, "")
			}
			continue
		}

		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID
		chatID := update.Message.Chat.ID
		userName := update.Message.From.FirstName

		if !isAdmin(userID, adminIDs) {
			continue
		}

		session := getSession(userID)
		if session != nil {
			switch session.Step {
			case "INPUT_INBOUND_TAG":
				session.Inbound = make(map[string]interface{})
				session.Inbound["tag"] = strings.TrimSpace(update.Message.Text)
				session.Inbound["remark"] = strings.TrimSpace(update.Message.Text)
				session.Step = "INPUT_INBOUND_PORT"
				setSession(userID, session)
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "🔌 پورت اینباند را وارد کنید (مثلا 443):")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}

			case "INPUT_INBOUND_PORT":
				portStr := strings.TrimSpace(update.Message.Text)
				port, err := strconv.Atoi(portStr)
				if err != nil || port <= 0 || port > 65535 {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ پورت نامعتبر. عدد بین 1 تا 65535 وارد کنید:")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				session.Inbound["listen_port"] = port
				session.Step = "INPUT_INBOUND_TYPE"
				setSession(userID, session)
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "📋 نوع اینباند را وارد کنید (مثلا vless, vmess):")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}

			case "INPUT_INBOUND_TYPE":
				inboundType := strings.TrimSpace(update.Message.Text)
				if inboundType == "" {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ نوع نامعتبر. دوباره وارد کنید:")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				session.Inbound["type"] = inboundType

				_, err := suiClient.SaveInbound("new", session.Inbound)
				if err != nil {
					log.Printf("SaveInbound error: %v", err)
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ خطا در ایجاد اینباند: %v", err))); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
				} else {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "✅ اینباند با موفقیت ایجاد شد!")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
				}
				setSession(userID, nil)
				sendMainMenu(bot, chatID, nil, userName)

			case "INPUT_NAME":
				name := strings.TrimSpace(update.Message.Text)
				if name == "" {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ نام نمی‌تواند خالی باشد. دوباره وارد کنید:")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				session.Name = name
				session.Step = "INPUT_VOLUME"
				setSession(userID, session)
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, "💾 حجم کاربر را به گیگابایت وارد کنید:")); err != nil {
					log.Printf("SendMessage error: %v", err)
				}

			case "INPUT_VOLUME":
				volStr := strings.TrimSpace(update.Message.Text)
				vol, err := strconv.Atoi(volStr)
				if err != nil || vol <= 0 {
					if _, err := bot.Send(tgbotapi.NewMessage(chatID, "❌ حجم نامعتبر. عدد مثبت وارد کنید:")); err != nil {
						log.Printf("SendMessage error: %v", err)
					}
					continue
				}
				session.Volume = vol
				session.Step = "CONFIRM"
				setSession(userID, session)
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📊 تایید اطلاعات:\nنام: %s\nحجم: %d GB\nاینباندها: %v\nمدت: ۳۰ روز", escapeMarkdown(session.Name), session.Volume, session.Inbounds))
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✅ تایید نهایی", "confirm_create")))
				if _, err := bot.Send(msg); err != nil {
					log.Printf("SendMessage error: %v", err)
				}
			}
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				sendMainMenu(bot, chatID, nil, userName)
			}
		}
	}
}