// http_fs project main.go
package main

import (
	"encoding/json"
	"flag"
	"github.com/kowsan/libsync/synclib"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	dirpath   string
	asdir     string
	addr      string
	use_csumm bool
)

type fsHandler struct {
}

func (fs *fsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/index.go" {
		log.Println("Begin build file structure")
		out := synclib.BuildFileStructure(dirpath, use_csumm)
		log.Println("End build file structure")
		log.Println("Begin send response file structure")
		b, _ := json.Marshal(out)

		w.WriteHeader(200)
		w.Write(b)
		log.Println("response sended")
	} else {
		http.ServeFile(w, r, dirpath+r.URL.Path)
	}

}

func main() {
	log.Println("Starting server")
	flag.StringVar(&addr, "addr", ":8181", "http addr (:port or host:port) ")
	flag.StringVar(&dirpath, "dirpath", "", "set directory to serve files ")
	flag.StringVar(&asdir, "asdir", "/", "serve as")
	flag.BoolVar(&use_csumm, "csumm", false, "Use file control summ")
	flag.Parse()

	dirpath = strings.TrimSuffix(dirpath, "/")
	log.Println("directory set to :", dirpath)

	log.Println("Serve as  :", asdir)
	log.Println("Csumm  :", use_csumm)
	myHandler := &fsHandler{}

	//fs := http.FileServer(http.Dir(dirpath))
	//	http.Handle(asdir, http.StripPrefix(asdir, fs))
	s := &http.Server{
		WriteTimeout: 50 * time.Minute,
		Addr:         addr,
		Handler:      myHandler,
	}

	http.HandleFunc(asdir, func(w http.ResponseWriter, r *http.Request) {

		http.ServeFile(w, r, dirpath+r.URL.Path)
	})
	//	http.HandleFunc(asdir+"index.go", func(w http.ResponseWriter, r *http.Request) {
	//		log.Println("Begin build file structure")
	//		out := synclib.BuildFileStructure(dirpath, use_csumm)
	//		log.Println("End build file structure")
	//		log.Println("Begin send response file structure")
	//		b, _ := json.Marshal(out)

	//		w.WriteHeader(200)
	//		w.Write(b)
	//		log.Println("response sended")
	//	})
	log.Println("start http server at : ", addr)
	s.ListenAndServe()

}
