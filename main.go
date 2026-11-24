// Command food-permit-license-renewal-tracker keeps every statutory permit a
// food business holds in one register, across the authorities that issue them
// and the different cycles they run on, and answers the question that matters
// before the shutters go up: is anything expired such that we are trading
// unlawfully today?
//
// It is one binary with no dependencies outside the Go standard library. It
// serves a small web interface on the loopback address and stores everything in
// a plain JSON file on the same machine. It makes no outbound network
// connection of any kind: there is no telemetry, no update check, and no link
// to any government portal.
//
// No certificate is ever uploaded or stored. Each permit carries a reference —
// a path or a note in the owner's own words — to where the paper itself is
// kept.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

//go:embed web
var webFiles embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on; loopback only unless you deliberately change it")
	data := flag.String("data", "./food-permit-license-renewal-tracker-data.json", "path to the local data file")
	flag.Parse()

	if err := run(*addr, *data); err != nil {
		log.Fatalf("food-permit-license-renewal-tracker: %v", err)
	}
}

func run(addr, data string) error {
	store, err := OpenStore(data)
	if err != nil {
		return err
	}

	ui, err := fs.Sub(webFiles, "web")
	if err != nil {
		return fmt.Errorf("could not open the embedded interface: %w", err)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           NewServer(store, ui),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	out := log.New(os.Stdout, "", 0)
	out.Printf("Food permit and licence renewal tracker")
	out.Printf("  open      http://%s", listener.Addr().String())
	out.Printf("  data file %s", store.Path())
	out.Printf("  offline   nothing is sent anywhere; stop with Ctrl-C")

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
