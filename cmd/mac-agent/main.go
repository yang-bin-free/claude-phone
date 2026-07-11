package main

import (
	"flag"
	"log"

	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

func main() {
	addr := flag.String("addr", engine.DefaultAddr, "HTTP/WebSocket listen address")
	claudeBin := flag.String("claude-bin", "claude", "Claude CLI binary")
	workdir := flag.String("workdir", ".", "default Claude working directory")
	permission := flag.String("permission", "default", "default Claude permission mode")
	flag.Parse()

	e := engine.New(engine.Config{
		Addr:              *addr,
		ClaudeBin:         *claudeBin,
		DefaultWorkingDir: *workdir,
		DefaultPermission: *permission,
	})
	log.Printf("claude-phone-agent listening on %s", *addr)
	if err := e.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
