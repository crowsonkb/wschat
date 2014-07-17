package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"go/build"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var flagAddress string
var flagAssetDir string

func init() {
	flag.StringVar(&flagAddress, "address", ":8080",
		"The HTTP address to bind to (e.g. ':8080'.")
	flag.StringVar(&flagAssetDir, "asset_dir", "",
		"The location of the static assets directory.")
}

func HandleChat(ws *websocket.Conn) {
	log.Printf("Connection opened: %s", ws.Request().RemoteAddr)
	defer log.Printf("Connection closed: %s", ws.Request().RemoteAddr)
	for {
		var msg string
		for msg == "" {
			if ws.SetReadDeadline(time.Now().Add(200*time.Millisecond)) != nil {
				return
			}
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				if err, ok := err.(*net.OpError); !ok || !err.Timeout() {
					return
				}
			}
		}
		if websocket.Message.Send(ws, msg) != nil {
			return
		}
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
	http.Handle("/", http.FileServer(http.Dir(flagAssetDir)))
	http.Handle("/chat", websocket.Handler(HandleChat))
	log.Fatal(http.ListenAndServe(flagAddress, nil))
}
