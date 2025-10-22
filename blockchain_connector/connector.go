package blockchainconnector

import (
	"42sol/BlockchainMonitoringBot/config"
	databaseconnector "42sol/BlockchainMonitoringBot/database_connector"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-telegram/bot"
)

// this one is to hold either parsed json response or scanned Node models sicne therir key elements would be identical
type TargetNodeDesc struct {
	ID               int    `json:"id"`
	Key              string `json:"key"`
	URI              string `json:"backend_uri"`
	BackendURIPublic string `json:"backend_uri_public"`
	IsReportedToday  bool
}

type GetHealthResponse struct {
	AMQP     AMQPStatus     `json:"amqp"`
	Database DatabaseStatus `json:"database"`
	Sign     SignStatus     `json:"sign"`
	Ethereum EthereumStatus `json:"ethereum"`
	Actions  ActionsStatus  `json:"actions"`
	Errors   []any          `json:"errors"`
}

type AMQPStatus struct {
	Enabled bool                 `json:"enabled"`
	Queues  map[string]QueueInfo `json:"queues"`
}

type QueueInfo struct {
	State     string `json:"state"`
	Consumers int    `json:"consumers"`
	Connected bool   `json:"connected"`
	Channel   bool   `json:"channel"`
}

type DatabaseStatus struct {
	Connected bool `json:"connected"`
}

type SignStatus struct {
	Certificate CertificateInfo `json:"certificate"`
}

type CertificateInfo struct {
	Hash     string `json:"hash"`
	IsValid  bool   `json:"is_valid"`
	Deadline int    `json:"deadline"`
}

type EthereumStatus struct {
	BlockNumber     string `json:"block_number"`
	BlockNumberDec  int64  `json:"block_number_dec"`
	Peers           int    `json:"peers"`
	RaftPeers       int    `json:"raft_peers"`
	MirrorBlockLast int64  `json:"mirror_block_last"`
	MirrorBlockDB   int64  `json:"mirror_block_db"`
}

type ActionsStatus struct {
	Deals ActionCounter `json:"deals"`
	Files ActionCounter `json:"files"`
}

type ActionCounter struct {
	Active int `json:"active"`
	Error  int `json:"error"`
	Hold   int `json:"hold"`
}

func GetHealth(nodeURL string, nodeName string) (string, error) {
	var healthData GetHealthResponse

	if !strings.Contains(nodeURL, "/health?full=1") {
		nodeURL = nodeURL + "/health?full=1"
	}

	response, err := http.Get(nodeURL)

	if err != nil {
		log.Println(err)
		return "", errors.New("API бэкенда недоступно или бэкенд не запущен")
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)

	if err != nil {
		log.Println(err)
		return "", errors.New("Не удаётся прочитать ответ API")
	}

	err = json.Unmarshal(body, &healthData)

	if err != nil {
		log.Println(err)
		return "", errors.New("Не удаётся прочитать ответ API")
	}

	return makeUserFriendlyReport(healthData, nodeName), nil
}

func makeUserFriendlyReport(response GetHealthResponse, nodeName string) string {
	//fmt.Println(response)

	var healthString string
	var ErrorIsFound = false

	if nodeName != "" {
		healthString = "Отчёт об узле " + nodeName + "\n"
	} else {
		healthString = "Отчёт об узле: \n"
	}

	if !response.Database.Connected {
		ErrorIsFound = true
		healthString += " - Отсутствует соединение бэкенда и базы данных или сервер базы данных не запущен/работает неправильно \n"
	}

	if response.AMQP.Enabled && (len(response.AMQP.Queues) == 0) {
		ErrorIsFound = true
		healthString += " - Отсутствует соединение бэкенда со службой RabbitMQ или сервер RabbitMQ не запущен/работает неправильно, или настройка AMQP на бэкенде произведена неправильно \n"
	}

	//fmt.Printf(response.AMQP.Queues["1s"].State)
	//fmt.Printf(string(response.AMQP.Queues["1S"].Consumers))
	//fmt.Printf(strconv.FormatBool(response.AMQP.Queues["1S"].Connected))
	//fmt.Printf(strconv.FormatBool(response.AMQP.Queues["1s"].Channel))

	if response.AMQP.Enabled && (response.AMQP.Queues["1s"].State != "running" ||
		response.AMQP.Queues["1S"].Consumers == 0 ||
		!response.AMQP.Queues["1S"].Connected ||
		!response.AMQP.Queues["1s"].Channel) {

		ErrorIsFound = true
		healthString += " - Проблемы с прослушиванием очереди 1S (входящие сообщения не обрабатываются бэкендом) \n"
	}

	if response.Sign.Certificate.Hash == "EMPTY" {
		ErrorIsFound = true
		healthString += " - Не задан сертификат для подписания документов (оплата по сделкам невозможна) \n"
	}

	if !response.Sign.Certificate.IsValid {
		ErrorIsFound = true
		healthString += " - Сертификат не прошел валидацию (оплата по сделкам невозможна) \n"
	}

	if response.Sign.Certificate.Deadline < 30 {
		healthString += fmt.Sprintf(" - Срок действия сертификат заканчивается через %d \n", response.Sign.Certificate.Deadline)
	}

	if response.Ethereum.BlockNumberDec == 0 {
		ErrorIsFound = true
		healthString += " - Отсутствует соединение с RPC сетью ethereum или служба ethereum не запущена/работает неправильно \n"
	}

	if response.Ethereum.Peers == 0 {
		ErrorIsFound = true
		healthString += " - Узел ethereum не подключен к другим узлам сети \n"
	}

	if response.Ethereum.RaftPeers == 0 {
		healthString += " - Проблемы с RAFT на узле ethereum \n"
	}

	if response.Ethereum.BlockNumberDec-response.Ethereum.MirrorBlockLast > 1000 {
		ErrorIsFound = true
		healthString += " - Адаптер mirror не запущен или не синхронизируется с сетью ethereum \n"
	}

	if response.Ethereum.BlockNumberDec-response.Ethereum.MirrorBlockDB > 1000 {
		ErrorIsFound = true
		healthString += " - Адаптер mirror не запущен или не производит обработку блоков сети ethereum \n"
	}

	if response.Actions.Deals.Error != 0 || response.Actions.Files.Error != 0 {
		ErrorIsFound = true
		healthString += " - Найдены проблемы в обработке сделок \n"
	}

	if len(response.Errors) != 0 {
		ErrorIsFound = true
		healthString += " - Имеются проблемы, выявленные самодиагностикой \n"
	}

	if !ErrorIsFound {
		healthString += " - Ошибок не выявлено \n"
	}

	return healthString
}

// Singular background health check run
func RunBackgroundHealthCheck(ctx context.Context, botObject *bot.Bot) {
	year, month, day := time.Now().Date()
	startOfToday := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	var notUpdatedToday, totalCount int64
	var nodesList []TargetNodeDesc

	db := databaseconnector.GetDBInstance()
	db.Model(databaseconnector.Node{}).Where("updated_at < ?", startOfToday).Count(&notUpdatedToday)
	db.Model(databaseconnector.Node{}).Count(&totalCount)

	if notUpdatedToday > 0 || totalCount == 0 {
		var nodesToCheckUrl string = os.Getenv("ADMIN_NODE") + "/node"

		response, err := http.Get(nodesToCheckUrl)

		if err != nil {
			// Admin node API cant be reached
			log.Println(err)
			return
		}

		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)

		if err != nil {
			log.Println(fmt.Println(err))
			return
		}

		err = json.Unmarshal(body, &nodesList)

		if err != nil {
			log.Println(err)
			return
		}
	} else {
		db.Model(databaseconnector.Node{}).Select("key", "uri", "is_reported_today").Scan(&nodesList)
	}

	sendReportToSubs(ctx, botObject, nodesList)
}

func sendReportToSubs(ctx context.Context, botObject *bot.Bot, nodesList []TargetNodeDesc) {
	db := databaseconnector.GetDBInstance()
	var chatIDs []string

	db.Model(databaseconnector.User{}).Where("chat_id != ?", nil).Pluck("chat_id", &chatIDs)

	for _, node := range nodesList {

		// do not report if report was already sent today
		//if node.IsReportedToday {
		//	continue
		//}

		touchOrCreateNode(node)
		healthReport, err := GetHealth(node.URI, node.Key)

		for _, chatID := range chatIDs {
			if err != nil {
				botObject.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   "Ошибка получения данных от узла " + node.Key,
				})
			} else {
				botObject.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   healthReport,
				})
			}
		}
	}
}

func touchOrCreateNode(node TargetNodeDesc) {
	var matchFound int64
	db := databaseconnector.GetDBInstance()
	db.Model(databaseconnector.Node{}).Where("key = ? AND uri = ?", node.Key, node.URI).Count(&matchFound)

	if matchFound > 0 {
		//db.Model(databaseconnector.Node{}).Where("key = ? AND uri = ?", node.Key, node.URI).Update("is_reported_today", true)
		db.Model(databaseconnector.Node{}).Where("key = ? AND uri = ?", node.Key, node.URI).Update("updated_at", time.Now())
	} else {
		db.Model(databaseconnector.Node{}).Create(databaseconnector.Node{Key: node.Key, Uri: node.URI}) // IsReportedToday: true
	}
}

func ScheduleHealthCheck(ctx context.Context, botObject *bot.Bot) {
	ticker := time.NewTicker(time.Duration(config.AppConfig.Bot.ScanIntervalMinute) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			RunBackgroundHealthCheck(ctx, botObject)
		case <-ctx.Done():
			return
		}
	}
}
