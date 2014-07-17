package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"fmt"
	"go/build"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const broadcasterQueueDepth = 10

var br *Broadcaster

var flagAddress string
var flagAssetsDir string
var flagTLSCert string
var flagTLSKey string

func init() {
	flag.StringVar(&flagAddress, "address", ":8080",
		"The HTTP address to bind to (e.g. ':8080'.")
	flag.StringVar(&flagAssetsDir, "assets_dir", "",
		"The location of the static assets directory.")
	flag.StringVar(&flagTLSCert, "tls_cert", "", "The TLS cert file.")
	flag.StringVar(&flagTLSKey, "tls_key", "", "The TLS key file.")
}

type Message struct {
	Content string
	User    string
	Time    time.Time
}

func (msg *Message) String() string {
	return fmt.Sprintf("%02d:%02d:%02d <%s> %s",
		msg.Time.Hour(),
		msg.Time.Minute(),
		msg.Time.Second(),
		msg.User,
		msg.Content)
}

type Broadcaster struct {
	sinks []chan Message
	lock  sync.Mutex
}

func NewBroadcaster() *Broadcaster {
	var br Broadcaster
	br.sinks = make([]chan Message, 0)
	return &br
}

func (br *Broadcaster) NewSink() chan Message {
	sink := make(chan Message, broadcasterQueueDepth)
	br.lock.Lock()
	defer br.lock.Unlock()
	br.sinks = append(br.sinks, sink)
	return sink
}

func (br *Broadcaster) DelSink(sink chan Message) {
	br.lock.Lock()
	defer br.lock.Unlock()

	for i := range br.sinks {
		if br.sinks[i] == sink {
			close(br.sinks[i])
			br.sinks = append(br.sinks[:i], br.sinks[i+1:]...)
			return
		}
	}
}

func (br *Broadcaster) Broadcast(msg Message) {
	br.lock.Lock()
	defer br.lock.Unlock()

	for _, sink := range br.sinks {
		select {
		case sink <- msg:
		default:
		}
	}
}

func HandleChat(ws *websocket.Conn) {
	log.Printf("Connection opened: %s", ws.Request().RemoteAddr)
	defer log.Printf("Connection closed: %s", ws.Request().RemoteAddr)

	sink := br.NewSink()
	defer br.DelSink(sink)

	go func() {
		defer br.DelSink(sink)
		for {
			var input string
			if websocket.Message.Receive(ws, &input) != nil {
				return
			}
			br.Broadcast(Message{
				Content: input,
				User:    ws.Request().RemoteAddr,
				Time:    time.Now()})
		}
	}()

	for msg := range sink {
		if websocket.Message.Send(ws, msg.String()) != nil {
			return
		}
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	flag.Parse()
	if flagAssetsDir == "" {
		pkg, err := build.Default.Import("github.com/crowsonkb/wschat", "", 0)
		if err != nil {
			log.Fatal("could not locate assets directory")
		}
		flagAssetsDir = pkg.Dir + "/assets"
	}
	fi, err := os.Stat(flagAssetsDir)
	if err != nil {
		log.Fatal(err)
	}
	if !fi.IsDir() {
		log.Fatal("assets_dir is not a directory")
	}
	br = NewBroadcaster()
	http.Handle("/", http.FileServer(http.Dir(flagAssetsDir)))
	http.Handle("/chat", websocket.Handler(HandleChat))
	switch {
	case flagTLSCert == "" && flagTLSKey == "":
		log.Fatal(http.ListenAndServe(flagAddress, nil))
	case flagTLSCert != "" && flagTLSKey != "":
		log.Fatal(http.ListenAndServeTLS(
			flagAddress, flagTLSCert, flagTLSKey, nil))
	default:
		log.Fatal("tls_cert and tls_key must both be provided")
	}
}
