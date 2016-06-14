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
	// log        *log.Logger
	pocketCDNs = groupcache.NewGroup("pocketCDN", 512<<20, groupcache.GetterFunc(
		func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
			filePath := key
			bytes, err := download(filepath)
			if err != nil {
				return err
			}
			dest.SetBytes(bytes)
			return nil
		}))
)

func download(key string) ([]byte, error) {
	u, _ := url.Parse(*resource)
	u.Path = key

	log.Printf("Requesting file %s from mirror site %s", key, *mirror)
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

	// encode using MessagePack format
	encoder := codec.NewEncoder(buf, &codec.MsgpackHandle{})
	err = encoder.Encode(HttpResponse{resp.Header, body})
	return buf.Bytes(), err
}

func InitSignal() {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			s := <-sig
			log.Printf("Received signal:", s)
			if state.IsClosed() {
				log.Printf("Cold close")
				os.Exit(1)
			}
			log.Printf("Warm close, waiting")
			go func() {
				state.Close()
				os.Exit(0)
			}()

		}
	}()

}

type HttpResponse struct {
	Header http.Header
	Body   []byte
}

func FileHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path

	state.addActiveDownload(1)
	defer state.addActiveDownload(-1)

	// file request hits master node
	if *master != "" {
		// peer is chosen randomly
		peer, err := peerGroup.PickPeer()
		if err != nil {
			log.Fatal(err)
		}
		peerURL, _ := url.Parse(peer)
		peerURL.Path = r.URL.Path
		peerURL.RawQuery = r.URL.RawQuery

		// Redirect the request to the chosen peer
		http.Redirect(w, r, peerURL.String(), http.StatusFound)
		return
	}

	// File request hits peer node
	log.Printf("key: %s has been request on peer %s", key, r.URL.Host)

	// Attempt to get data from group cache
	var data []byte
	var ctx groupcache.Context
	err := pocketCDNs.Get(ctx, key, groupcache.AllocatingByteSliceSink(&data))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var response HttpResponse
	decoder := codec.NewDecoder(bytes.NewReader(data), &codec.MsgpackHandle{})
	err = decoder.Decode(&response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// FIXME: writing response back to requester
	for key, _ := range response.Header {
		w.Header().Set(key, responseHeader.Get(key))
	}

	// FIXME: appears this crap is for logging purpose
	// why do we need to send this senddata crap
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

	if *resource != "" {
		sendc <- sendData
	}

	// serving file
	var modTime time.Time = time.Now()
	rd := bytes.NewReader(response.Body)
	http.ServeContent(w, r, filepath.Base(key), modTime, rd)
}

func LogHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, *logfile)
}

func main() {
	var (
		resource = flag.String("resource", "localhost:5000", "resource node that serves end files")
		logfile  = flag.String("log", "cdn.log", "log file location")
		master   = flag.String("master", "localhost:8001", "master node address")
		peer     = flag.String("peer", "", "peer node address")
	)
	flag.Parse()

	switch {
	case *master != "":
		if err := InitMaster(); err != nil {
			log.Fatal(err)
		}
	case *peer != "":
		if err := InitPeer(); err != nil {
			log.Fatal(err)
		}
	}

	http.HandleFunc("/", FileHandler)
	http.HandleFunc("/_log", LogHandler)
	log.Fatal(http.ListenAndServe(), nil)
}
