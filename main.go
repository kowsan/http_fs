// http_fs project main.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/kowsan/libsync/synclib"
)

var (
	dirpath   string
	asdir     string
	addr      string
	use_csumm bool
)

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
	fs := http.FileServer(http.Dir(dirpath))
	http.Handle(asdir, http.StripPrefix(asdir, fs))
	http.HandleFunc(asdir+"index.go", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Begin build file structure")
		out := synclib.BuildFileStructure(dirpath, use_csumm)
		log.Println("End build file structure")
		log.Println("Begin send response file structure")
		b, _ := json.Marshal(out)

		w.WriteHeader(200)
		w.Write(b)
		log.Println("response sended")
	})
	log.Println("start http server at : ", addr)
	http.ListenAndServe(addr, nil)

}
