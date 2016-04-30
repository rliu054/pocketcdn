package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	mirror   = flag.String("mirror", "", "Mirror web base URL")
	logfile  = flag.String("log", "-", "Set log file, default stdout")
	upstream = flag.String("upstream", "", "server base URL, conflict with -mirror")
	address  = flag.String("addr", ":5000", "Listen address")
	token    = flag.String("token", "", "peer and master token should be same")
)

func main() {
	flag.Parse()
	http.HandleFunc("/", FileHandler)
	http.HandleFunc("/_log", LogHandler)

	log.Printf("Listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address))
}
