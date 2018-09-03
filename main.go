package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	ws "github.com/gorilla/websocket"
)

var (
	port     int
	from, to []string
	logger   *log.Logger

	symbols = map[string]string{
		"EUR": "€",
		"BTC": "₿",
		"ETH": "Ξ",
		"LTC": "Ł",
	}
)

func init() {
	flag.IntVar(&port, "port", 15000, "Port of the shepherd")
	sFrom := flag.String("from", "", "Base currency")
	sTo := flag.String("to", "", "Target currency")

	flag.Parse()

	if *sFrom == "" || *sTo == "" {
		log.Fatalln("Missing from/to currency.")
	}

	from = strings.Split(*sFrom, ",")
	to = strings.Split(*sTo, ",")

	if len(from) != len(to) {
		log.Fatalln("from/to don't match.")
	}

	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

func main() {
	var (
		wsDialer ws.Dialer
		wsConn   *ws.Conn
	)
	for {
		var err error
		wsConn, _, err = wsDialer.Dial("wss://ws-feed.gdax.com", nil)
		if err == nil {
			break
		}

		logger.Println(err)

		time.Sleep(time.Second * 5)
	}

	for i := 0; i < len(from); i++ {
		id := fmt.Sprintf("indicator-gdax-%s-%s", from[i], to[i])
		label := fmt.Sprintf("%s/%s: N/A", symbol(from[i]), symbol(to[i]))

		if err := update(id, label); err != nil {
			logger.Println(err)
		}
	}

	productIds := make([]string, len(from))
	for i := 0; i < len(from); i++ {
		productIds[i] = fmt.Sprintf("%s-%s", from[i], to[i])
	}

	subscribe := &Subscription{
		Type:       "subscribe",
		ProductIds: productIds,
		Channels:   []string{"ticker"},
	}
	if err := wsConn.WriteJSON(subscribe); err != nil {
		logger.Fatalln(err)
	}

	for {
		msg := &Ticker{}
		if err := wsConn.ReadJSON(msg); err != nil {
			logger.Println(err)
			continue
		}

		if msg.ProductId == "" {
			continue
		}

		logger.Printf("Got %.2f (%s, %.6f)", msg.Price, msg.ProductId, msg.LastSize)

		product := strings.Split(msg.ProductId, "-")

		id := fmt.Sprintf("indicator-gdax-%s-%s", product[0], product[1])
		label := fmt.Sprintf("%s/%s: %.0f", symbol(product[0]), symbol(product[1]), msg.Price)
		if err := update(id, label); err != nil {
			logger.Println(err)
		}
	}
}

func update(id, label string) error {
	resp, err := http.Post(fmt.Sprintf("http://localhost:%v/%s", port, id), "text/plain", strings.NewReader(label))
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func symbol(currency string) string {
	if s, ok := symbols[currency]; ok {
		return s
	}

	return currency
}

type Subscription struct {
	Type       string   `json:"type"`
	ProductIds []string `json:"product_ids"`
	Channels   []string `json:"channels"`
}

type Ticker struct {
	ProductId string  `json:"product_id"`
	Price     float64 `json:"price,string"`
	LastSize  float64 `json:"last_size,string"`
}
