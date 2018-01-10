package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	ws "github.com/gorilla/websocket"
	pb "github.com/loamhoof/indicator"
	"github.com/loamhoof/indicator/client"
)

var (
	icon, sFrom, sTo, logFile string
	port                      int
	from, to                  []string
	logger                    *log.Logger

	symbols = map[string]string{
		"EUR": "€",
		"BTC": "₿",
		"ETH": "Ξ",
	}
)

func init() {
	flag.IntVar(&port, "port", 15000, "Port of the shepherd")
	flag.StringVar(&icon, "icon", "", "Path to the icon")
	flag.StringVar(&sFrom, "from", "", "Base currency")
	flag.StringVar(&sTo, "to", "", "Target currency")
	flag.StringVar(&logFile, "log", "", "Log file")

	flag.Parse()

	if sFrom == "" || sTo == "" {
		log.Fatalln("Missing from/to currency.")
	}

	from = strings.Split(sFrom, ",")
	to = strings.Split(sTo, ",")

	if len(from) != len(to) {
		log.Fatalln("from/to don't match.")
	}

	logger = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

func main() {
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			logger.Fatalln(err)
		}
		defer f.Close()
		logger = log.New(f, "", log.LstdFlags)
	}

	sc := client.NewShepherdClient(port)
	err := sc.Init()
	if err != nil {
		logger.Fatalln(err)
	}
	defer sc.Close()

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
		iReq := &pb.Request{
			Id:         fmt.Sprintf("indicator-gdax-%s-%s", from[i], to[i]),
			Icon:       icon,
			Label:      fmt.Sprintf("%s/%s: N/A", symbol(from[i]), symbol(to[i])),
			LabelGuide: "AAA/BBB: 123456789",
			Active:     true,
		}

		if _, err := sc.Update(iReq); err != nil {
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

		iReq := &pb.Request{
			Id:         fmt.Sprintf("indicator-gdax-%s-%s", product[0], product[1]),
			Icon:       icon,
			Label:      fmt.Sprintf("%s/%s: %.0f", symbol(product[0]), symbol(product[1]), msg.Price),
			LabelGuide: "AAA/BBB: 123456789",
			Active:     true,
		}
		if _, err := sc.Update(iReq); err != nil {
			logger.Println(err)
		}
	}
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
