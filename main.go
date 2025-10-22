package main

import (
	blockchainConnector "42sol/BlockchainMonitoringBot/blockchain_connector"
	"42sol/BlockchainMonitoringBot/config"
	databaseConnector "42sol/BlockchainMonitoringBot/database_connector"
	telegramConnector "42sol/BlockchainMonitoringBot/telegram_connector"
	"log"

	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

func main() {
	// load env and config ----------------
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("err loading: %v", err)
	}

	config.Load()
	// ----------------

	// redirect logging to file ----------------
	log.SetOutput(&lumberjack.Logger{
		Filename:   "log.log",
		MaxSize:    5,
		MaxBackups: config.AppConfig.Logger.MaxLogs,
		MaxAge:     14,
		Compress:   false,
	})
	// ----------------

	// run bot ----------------
	databaseConnector.ResolveDbConnection()

	botContext, botObject, cancelFunc := telegramConnector.MakeBot()

	defer cancelFunc()

	go blockchainConnector.ScheduleHealthCheck(botContext, botObject)

	telegramConnector.RunBot(botContext, botObject)
}
