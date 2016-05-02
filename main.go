package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/codeskyblue/groupcache"
	"github.com/ugorji/go/codec"
)

var (
	cdnlog     *log.Logger
	thumbNails = groupcache.NewGroup("thumbnail", 512<<20, groupcache.GetterFunc(
		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
			fileName := key
			bytes, err := generateThumbnail(fileName)
			if err != nil {
				return err
			}
			dest.SetBytes(bytes)
			return nil
		}))
)

type HttpResponse struct {
	Header http.Header
	Body   []byte
}

func generateThumbnail(key string) ([]byte, error) {
	u, _ := url.Parse(*mirror)
	u.Path = key
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := bytes.NewBuffer(nil)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
}

func InitSignal() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			s := <-sig
			fmt.Println("Received signal:", s)
			if state.IsClosed() {
				fmt.Println("Cold close")
				os.Exit(1)
			}
			fmt.Println("Warm close, waiting")
			go func() {
				state.Close()
				os.Exit(0)
			}()

		}
	}()

}

func FileHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.path

	state.addActiveDownload(1)
	defer state.addActiveDownload(-1)

	// if master node is hit, redirect to one of peer nodes
	if *upstream == "" {
		if peerAddr, err := peerGroup.PeekPeer(); err == nil {
			u, _ := url.Parse(peerAddr)
			u.Path = r.URL.Path
			u.RawQuery = r.URL.RawQuery
			http.Redirect(w, r, u.String(), http.StatusFound)
			return
		}
	}
	fmt.Println("KEY:", key)
	var data []byte
	var ctx groupcache.Context
	err := thumbNails.Get(ctx, key, groupcache.AllocatingByteSliceSink(&data))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var response HttpResponse
	decoder := codec.NewDecoder(bytes.NewReader(data), &codec.MsgpackHandle{})
	err = decoder.Decode(&response)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// FIXME: should be improved
	for key, _ := range response.Header {
		w.Header().Set(key, responseHeader.Get(key))
	}

	sendData := map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"key":         key,
		"success":     err == nil,
		"user_agent":  r.Header.Get("User-Agent"),
	}
	headerData := r.Header.Get("X-Pocketcdn-Data")
	headerType := r.Header.Get("X-Pocketcdn-Type")
	if headerType == "json" {
		var data interface{}
		err := json.Unmarshal([]byte(headerData), &data)
		if err == nil {
			sendData["header_data"] = data
			sendData["header_type"] = headerType
		} else {
			log.Println("header data decode:", err)
		}
	} else {
		sendData["header_data"] = headerData
		sendData["header_type"] = headerType
	}

	if *upstream != "" {
		sendc <- sendData
	}

	// FIXME: modtiem should come from header
	var modTime time.Time = time.Now()

	rd := bytes.NewReader(response.Body)
	http.ServeContent(w, r, filepath.Base(key), modTime, rd)
}

func LogHandler(w http.ResponseWriter, r *http.Request) {
	if *logfile == "" || *logfile == "-" {
		http.Error(w, "Log file not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, *logfile)
}

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
	log.Fatal(http.ListenAndServe(), nil)
}
