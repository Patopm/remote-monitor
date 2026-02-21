package main

import (
	"log"
	"net/http"

	mw "github.com/Patopm/remote-monitor/internal/middleware"
)

func main() {
	hub := mw.NewHub()

	mux := http.NewServeMux()
	mw.RegisterRoutes(mux, hub)

	handler := mw.CORSMiddleware(mux)

	addr := ":8080"
	log.Printf("[Middleware] Listening on %s", addr)
	log.Printf("[Middleware] WebSocket endpoint: ws://localhost%s/ws/agent", addr)
	log.Printf("[Middleware] REST API:           http://localhost%s/api/", addr)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("[Middleware] Server failed: %v", err)
	}
}
