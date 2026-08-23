// Command serve hosts the browser demo: the built wasm modules plus the page
// and Go's wasm_exec.js glue, all from one directory.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "listen address")

	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	build := filepath.Join(root, "build")
	host := filepath.Join(root, "host")

	goroot, err := exec.CommandContext(context.Background(), "go", "env", "GOROOT").Output()
	if err != nil {
		log.Fatal(err)
	}
	glue := filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js")

	mux := http.NewServeMux()
	mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		http.ServeFile(w, r, glue)
	})
	// `just site` flattens host/ and build/ into one directory, so serve them
	// as one here too: the page's relative fetches must resolve the same way in
	// development as they do on Pages.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(host, "index.html"))
			return
		}

		name := filepath.Join(host, filepath.Base(r.URL.Path))

		_, statErr := os.Stat(name)
		if statErr == nil {
			http.ServeFile(w, r, name)
			return
		}

		http.FileServer(http.Dir(build)).ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("serving http://%s", *addr)
	log.Fatal(srv.ListenAndServe())
}
