package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/gorilla/websocket"
	"github.com/gosuri/uilive"
	"github.com/guptarohit/asciigraph"
)

// Создаем срезы для хранения данных по BTC_USD и LTC_USD
var btcUsd []Data
var ltcUsd []Data
var ethUsd []Data

var btcUsdF = []float64{0}
var ltcUsdF = []float64{0}
var ethUsdF = []float64{0}

var btcUsdL float64
var ltcUsdL float64
var ethUsdL float64

func UnmarshalWelcome(data []byte) (Welcome, error) {
	var r Welcome
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Welcome) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Welcome struct {
	Ts    int64  `json:"ts"`
	Event string `json:"event"`
	Topic string `json:"topic"`
	Data  Data   `json:"data"`
}

type Data struct {
	BuyPrice  string `json:"buy_price"`
	SellPrice string `json:"sell_price"`
	LastTrade string `json:"last_trade"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Avg       string `json:"avg"`
	Vol       string `json:"vol"`
	VolCurr   string `json:"vol_curr"`
	Updated   int64  `json:"updated"`
}

func mainMenu() {
	fmt.Println("My menu:")
	fmt.Println("1. BTC_USD")
	fmt.Println("2. LTC_USD")
	fmt.Println("3. ETH_USD")
	fmt.Println("Press q to quit")
}

func submenu1() {
	fmt.Printf("BTC_USD: %f", btcUsdL)
	go printGraph(&btcUsdF)
}

func submenu2() {
	fmt.Printf("LTC_USD: %f", ltcUsdL)
	go printGraph(&ltcUsdF)
}

func submenu3() {
	fmt.Printf("ETH_USD: %f", ethUsdL)
	go printGraph(&ethUsdF)
}

func printGraph(data *[]float64) {
	writer := uilive.New()
	writer.Start()
	for {
		//fmt.Print("\033[H\033[2J")
		graph := asciigraph.Plot(*data)
		fmt.Fprintln(writer, graph)
		time.Sleep(1 * time.Second)
		// Обновляем вывод
		writer.Flush()
	}
}

func main() {

	publicApiUsageExample(&btcUsd, &ltcUsd, &ethUsd)

	for _, price := range btcUsd {
		f64, _ := strconv.ParseFloat(price.BuyPrice, 64)
		btcUsdF = append(btcUsdF, f64)
		btcUsdL = f64
	}
	for _, price := range ltcUsd {
		f64, _ := strconv.ParseFloat(price.BuyPrice, 64)
		ltcUsdF = append(ltcUsdF, f64)
		ltcUsdL = f64
	}
	for _, price := range ethUsd {
		f64, _ := strconv.ParseFloat(price.BuyPrice, 64)
		ethUsdF = append(ethUsdF, f64)
		ethUsdL = f64
	}
	fmt.Println("BTC_USD:", btcUsdF, "LTC_USD:", ltcUsdF, "ETH_USD:", ethUsdF)

	// Получаем канал событий клавиатуры и ошибку
	keysEvents, err := keyboard.GetKeys(10)
	if err != nil {
		// В случае ошибки выводим её и завершаем программу
		panic(err)
	}

	// Отложенная функция, которая будет вызвана при завершении работы main
	defer func() {
		// Закрываем клавиатурный поток
		_ = keyboard.Close()
	}()

	// Выводим инструкцию для пользователя

	currentMenu := mainMenu

	mainMenu()
	for {
		// Получаем следующее событие клавиатуры из канала
		event := <-keysEvents

		// Проверяем наличие ошибки при получении события
		if event.Err != nil {
			// В случае ошибки выводим её и завершаем программу
			panic(event.Err)
		}

		if event.Key == keyboard.KeyBackspace2 {
			currentMenu = mainMenu
		}

		switch event.Rune {
		case '1':
			currentMenu = submenu1
		case '2':
			currentMenu = submenu2
		case '3':
			currentMenu = submenu3
		case 'q':
			fmt.Println("BTC_USD:", btcUsdF, "LTC_USD:", ltcUsdF, "ETH_USD:", ethUsdF)
			return
		}

		// Clear the screen
		fmt.Print("\033[H\033[2J")

		// Display the current menu
		currentMenu()
	}
}

func publicApiUsageExample(btcUsd, ltcUsd, ethUsd *[]Data) {
	startExmoClient("wss://ws-api.exmo.com:443/v1/public", []string{
		`{"id":1,"method":"subscribe","topics":["spot/ticker:BTC_USD","spot/ticker:LTC_USD","spot/ticker:ETH_USD"]}`,
	}, btcUsd, ltcUsd, ethUsd)
}

func startExmoClient(url string, initMessages []string, btcUsd, ltcUsd, ethUsd *[]Data) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	// Каналы для обновлений цен
	btcUsdUpdates := make(chan Data)
	ltcUsdUpdates := make(chan Data)
	ethUsdUpdates := make(chan Data)

	done := make(chan struct{})

	// Запускаем горутины для воркеров с передачей срезов
	go priceWorker("BTC_USD", btcUsdUpdates, btcUsd)
	go priceWorker("LTC_USD", ltcUsdUpdates, ltcUsd)
	go priceWorker("ETH_USD", ethUsdUpdates, ethUsd)

	go func() {
		defer close(done)
		for {
			// Получение ответа от сервера
			_, message, err := c.ReadMessage()
			if err != nil {
				//log.Println("read:", err)
				return
			}

			// Парсинг JSON-сообщения и получение BuyPrice
			welcome, err := UnmarshalWelcome(message)
			if err != nil {
				log.Println("json unmarshal:", err)
				continue
			}

			// Вывод BuyPrice только если есть данные
			if welcome.Event == "update" && welcome.Data.BuyPrice != "" {
				// Отправка данных в канал для соответствующей торговой пары
				switch welcome.Topic {
				case "spot/ticker:BTC_USD":
					btcUsdUpdates <- welcome.Data
				case "spot/ticker:LTC_USD":
					ltcUsdUpdates <- welcome.Data
				case "spot/ticker:ETH_USD":
					ethUsdUpdates <- welcome.Data
				}
			}
		}
	}()

	for _, initMessage := range initMessages {
		err = c.WriteMessage(websocket.TextMessage, []byte(initMessage))
		if err != nil {
			log.Printf("fail to write init message: %v", err)
			return
		}
	}

	select {
	case <-interrupt:
		//log.Println("interrupt")

		// Cleanly close the connection by sending a close message and then
		// waiting (with timeout) for the server to close the connection.
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	case <-done:
		//default:
	}
}

func priceWorker(pair string, updates <-chan Data, dataSlice *[]Data) {
	for update := range updates {
		// Воркер обрабатывает данные о ценах на актив и выполняет необходимые действия
		//log.Printf("%s Price Update - Buy Price: %s", pair, update.BuyPrice)
		_ = pair
		// Добавляем обновленные данные в срез
		*dataSlice = append(*dataSlice, update)
	}
}
