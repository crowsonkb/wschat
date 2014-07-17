package main

import (
	//"code.google.com/p/go.net/websocket"
	"flag"
	"go/build"
	"log"
	"net/http"
	"os"
)

var flagAddress string
var flagAssetDir string

func init() {
	flag.StringVar(&flagAddress, "address", ":80",
		"The HTTP address to bind to (e.g. ':80'.")
	flag.StringVar(&flagAssetDir, "asset_dir", "",
		"The location of the static assets directory.")
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
	log.Fatal(http.ListenAndServe(flagAddress, nil))
}
