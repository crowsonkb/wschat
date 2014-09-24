package main

import (
	"code.google.com/p/go.net/websocket"
	"expvar"
	"flag"
	"fmt"
	"go/build"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	BroadcasterQueueDepth = 10
	LogSize               = 30
)

var (
	br  *Broadcaster
	lgr *Logger

	flagAddress   string
	flagAssetsDir string
	flagTLSCert   string
	flagTLSKey    string

	varClients  *expvar.Int
	varMsgsDrop *expvar.Int
	varMsgsIn   *expvar.Int
	varMsgsOut  *expvar.Int
)

func init() {
	flag.StringVar(&flagAddress, "address", ":8080",
		"The HTTP address to bind to (e.g. ':8080'.")
	flag.StringVar(&flagAssetsDir, "assets-dir", "",
		"The location of the static assets directory.")
	flag.StringVar(&flagTLSCert, "tls-cert", "", "The TLS cert file.")
	flag.StringVar(&flagTLSKey, "tls-key", "", "The TLS key file.")

	varClients = expvar.NewInt("Clients")
	varMsgsDrop = expvar.NewInt("MsgsDrop")
	varMsgsIn = expvar.NewInt("MsgsIn")
	varMsgsOut = expvar.NewInt("MsgsOut")
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
	sink := make(chan Message, BroadcasterQueueDepth)
	br.lock.Lock()
	defer br.lock.Unlock()
	br.sinks = append(br.sinks, sink)
	varClients.Set(int64(len(br.sinks)))
	return sink
}

func (br *Broadcaster) DelSink(sink chan Message) {
	br.lock.Lock()
	defer br.lock.Unlock()

	for i := range br.sinks {
		if br.sinks[i] == sink {
			close(br.sinks[i])
			br.sinks = append(br.sinks[:i], br.sinks[i+1:]...)
			varClients.Set(int64(len(br.sinks)))
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
			varMsgsDrop.Add(1)
		}
	}
}

type Logger struct {
	msgs []Message
	lock sync.Mutex
	sink chan Message
}

func NewLogger() *Logger {
	var lgr Logger
	lgr.msgs = make([]Message, 0, LogSize)
	lgr.sink = br.NewSink()
	go lgr.doLogging()
	return &lgr
}

func (lgr *Logger) doLogging() {
	for msg := range lgr.sink {
		lgr.lock.Lock()
		if len(lgr.msgs) < cap(lgr.msgs) {
			lgr.msgs = append(lgr.msgs, msg)
		} else {
			copy(lgr.msgs[:len(lgr.msgs)-1], lgr.msgs[1:])
			lgr.msgs[len(lgr.msgs)-1] = msg
		}
		lgr.lock.Unlock()
	}
}

func (lgr *Logger) GetLog() []Message {
	lgr.lock.Lock()
	defer lgr.lock.Unlock()
	msgs := make([]Message, len(lgr.msgs))
	copy(msgs, lgr.msgs)
	return msgs
}

func HandleChat(ws *websocket.Conn) {
	remoteAddr := ws.Request().RemoteAddr
	log.Printf("Connection opened: %s", remoteAddr)
	defer log.Printf("Connection closed: %s", remoteAddr)

	sink := br.NewSink()
	defer br.DelSink(sink)

	go func() {
		defer br.DelSink(sink)
		for {
			var input string
			if websocket.Message.Receive(ws, &input) != nil {
				return
			}
			varMsgsIn.Add(1)
			br.Broadcast(Message{
				Content: input,
				User:    remoteAddr,
				Time:    time.Now()})
		}
	}()

	for _, msg := range lgr.GetLog() {
		if websocket.Message.Send(ws, msg.String()) != nil {
			return
		}
		varMsgsOut.Add(1)
	}

	for msg := range sink {
		if websocket.Message.Send(ws, msg.String()) != nil {
			return
		}
		varMsgsOut.Add(1)
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
		log.Fatal("-assets-dir is not a directory")
	}

	br = NewBroadcaster()
	lgr = NewLogger()
	http.Handle("/", http.FileServer(http.Dir(flagAssetsDir)))
	http.Handle("/chat", websocket.Handler(HandleChat))
	switch {
	case flagTLSCert == "" && flagTLSKey == "":
		log.Fatal(http.ListenAndServe(flagAddress, nil))
	case flagTLSCert != "" && flagTLSKey != "":
		log.Fatal(http.ListenAndServeTLS(
			flagAddress, flagTLSCert, flagTLSKey, nil))
	default:
		log.Fatal("-tls-cert and -tls-key must both be provided")
	}
}
