package main

import (
	blockchainConnector "42sol/BlockchainMonitoringBot/blockchain_connector"
	databaseConnector "42sol/BlockchainMonitoringBot/database_connector"
	telegramConnector "42sol/BlockchainMonitoringBot/telegram_connector"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// make error log file
	f, err := os.OpenFile("log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	err = godotenv.Load()
	if err != nil {
		log.Fatalf("err loading: %v", err)
	}

	databaseConnector.ResolveDbConnection()

	botContext, botObject, cancelFunc := telegramConnector.MakeBot()

	defer cancelFunc()

	go blockchainConnector.ScheduleHealthCheck(botContext, botObject)

	telegramConnector.RunBot(botContext, botObject)
}
