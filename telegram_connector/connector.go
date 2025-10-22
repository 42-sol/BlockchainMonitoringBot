package telegramconnector

import (
	apiConnector "42sol/BlockchainMonitoringBot/blockchain_connector"
	databaseconnector "42sol/BlockchainMonitoringBot/database_connector"
	"context"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func MakeBot() (ctx context.Context, botObject *bot.Bot, cancel context.CancelFunc) {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt)

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	token := os.Getenv("TOKEN")

	botObject, err := bot.New(token, opts...)

	if err != nil {
		log.Fatal(err.Error())
		panic(err)
	}

	return ctx, botObject, cancelFunc
}

func RunBot(ctx context.Context, botObject *bot.Bot) {

	// hande "/help"
	botObject.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommand, helpCommandHandler)
	// handle "/register"
	botObject.RegisterHandler(bot.HandlerTypeMessageText, "/register", bot.MatchTypePrefix, registerCommandHandler, isAuthorizedToUseRegisterMiddlvare)
	//handle "/getHealth"
	botObject.RegisterHandler(bot.HandlerTypeMessageText, "/getHealth", bot.MatchTypePrefix, getHealthCommandHandler, isAuthorizedToUseRegisterMiddlvare)
	// handle "/start"
	botObject.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, startCommandHandler, isAuthorizedToUseRegisterMiddlvare)

	botObject.Start(ctx)
}

// handle any message beyond predetermined
func defaultHandler(ctx context.Context, botObject *bot.Bot, update *models.Update) {
	botObject.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Команда не распознана. Используйте /help для получения списка команд.",
	})
}

// handle /start command
func startCommandHandler(ctx context.Context, botObj *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	db := databaseconnector.GetDBInstance()
	db.Model(databaseconnector.User{}).Where("username = ?", update.Message.From.Username).Update("chat_id", chatID)

	botObj.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Бот активирован. Теперь вы периодически будуте получать уведомления о состоянии узлов.",
	})
}

// handle /help command
func helpCommandHandler(ctx context.Context, botObject *bot.Bot, update *models.Update) {
	_, err := botObject.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: "Бот обеспечивает мониторинг узлов 42sol/blockchain \n" +
			"Доступные команды: \n" +
			"- /help : отображае это сообщение \n" +
			"- /register *username* : предоставляет указанному пользователю доступ к боту \n" +
			"- /getHealth *url*: предоставляет отчёт о состоянии конкретного узла \n" +
			"- /start : добавляет вас в список получателей отчётов автоматического мониторинга",
	})

	if err != nil {
		log.Fatalln(err)
	}
}

// handle /register command
func registerCommandHandler(ctx context.Context, botObject *bot.Bot, update *models.Update) {
	userToRegister := strings.Trim(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/register")), "@")
	var errorMessage string

	if userToRegister != "" {
		db := databaseconnector.GetDBInstance()
		var count int64 // "does exist" indicator
		err := db.Model(databaseconnector.User{}).Where("username = ?", userToRegister).Count(&count).Error

		if err != nil {
			errorMessage = "Не удаётся добавить пользователя."
			SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)

			return
		}

		if count > 0 {
			errorMessage = "Пользователь уже был добавлен ранее"
			SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)

			return
		}

		err = db.Model(&databaseconnector.User{}).Create(&databaseconnector.User{Username: userToRegister}).Error

		if err != nil {
			var errorMessage string = "Не удаётся добавить пользователя"
			SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)

			return
		}

		botObject.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Пользователь " + update.Message.From.FirstName + " " + update.Message.From.LastName + " теперь имеет доступ к боту.",
		})

		return
	}

	errorMessage = "Параметр 'username' не может быть пустым"
	SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)
}

// handle /getHealth command
func getHealthCommandHandler(ctx context.Context, botObject *bot.Bot, update *models.Update) {
	url := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/getHealth"))

	healthMessage, err := apiConnector.GetHealth(url, "") // no node name in response

	if err != nil {
		SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, err.Error())
	}

	botObject.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   healthMessage,
	})
}

// check if user is authorized to run /register
func isAuthorizedToUseRegisterMiddlvare(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botObject *bot.Bot, update *models.Update) {
		var errorMessage string
		user := update.Message.From.Username

		db := databaseconnector.GetDBInstance()

		var count int64 // "does exist indicator"
		err := db.Model(databaseconnector.User{}).Where("username = ?", user).Count(&count).Error

		if err != nil {
			errorMessage = "Не удаётся проверить наличие доступа. В доступе отказано."
			SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)

			return
		}

		if count > 0 {
			next(ctx, botObject, update)
			return
		}

		errorMessage = "У вас недостаточно прав для использования этой команды. Свяжитесь с администратором для получения доступа."
		SendErrorOccurredMessage(botObject, ctx, update.Message.Chat.ID, errorMessage)
	}
}

// handling errors
func SendErrorOccurredMessage(botObject *bot.Bot, ctx context.Context, chatId int64, message string) {
	botObject.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatId,
		Text:   message,
		//MessageEffectID: "5046589136895476101", // poop
	})
}
