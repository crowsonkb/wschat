package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"go/build"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const broadcasterQueueDepth = 10

var br *Broadcaster

var flagAddress string
var flagAssetDir string

func init() {
	flag.StringVar(&flagAddress, "address", ":8080",
		"The HTTP address to bind to (e.g. ':8080'.")
	flag.StringVar(&flagAssetDir, "asset_dir", "",
		"The location of the static assets directory.")
}

type Message string

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

	for {
		input := ""
		for input == "" {
			// Handle queued messages from other users each time through this
			// loop.
		HandleQueue:
			for {
				select {
				case msg := <-sink:
					if websocket.Message.Send(ws, string(msg)) != nil {
						return
					}
				default:
					break HandleQueue
				}
			}

			// Handle one incoming message from the current user; if more than
			// 200 milliseconds pass without receiving one, loop back and check
			// if there are queued messages from other users.
			if ws.SetReadDeadline(time.Now().Add(200*time.Millisecond)) != nil {
				return
			}
			if err := websocket.Message.Receive(ws, &input); err != nil {
				if err, ok := err.(*net.OpError); !ok || !err.Timeout() {
					return
				}
			}
		}
		br.Broadcast(Message(input))
	}
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	flag.Parse()
	if flagAssetDir == "" {
		pkg, err := build.Default.Import("wschat", "", 0)
		if err != nil {
			log.Fatal("could not locate assets directory")
		}
		flagAssetDir = pkg.Dir + "/assets"
	}
	fi, err := os.Stat(flagAssetDir)
	if err != nil {
		log.Fatal(err)
	}
	if !fi.IsDir() {
		log.Fatal("asset_dir is not a directory")
	}
	br = NewBroadcaster()
	http.Handle("/", http.FileServer(http.Dir(flagAssetDir)))
	http.Handle("/chat", websocket.Handler(HandleChat))
	log.Fatal(http.ListenAndServe(flagAddress, nil))
}
