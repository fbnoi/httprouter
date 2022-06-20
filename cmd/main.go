package main

import (
	"log"
	"net/http"

	xrouter "fbnoi.com/httprouter"
)

func main() {
	router := xrouter.NewRouteTree(&xrouter.Config{
		RedirectFixedPath: true,
	})

	router.GET("index", "/", func(r *http.Request, w http.ResponseWriter, ps xrouter.Params) {
		w.Write([]byte("hello world"))
	}).GET("hello_world", "/hello/world", func(r *http.Request, w http.ResponseWriter, ps xrouter.Params) {
		w.Write([]byte("hello"))
	}).GET("test", "/:test", func(r *http.Request, w http.ResponseWriter, ps xrouter.Params) {
		w.Write([]byte("123"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())
}
