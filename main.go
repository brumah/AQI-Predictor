package main

import (
	"log"
	"net/http"
)

var (
	dataChannel chan Data = make(chan Data, 1)
)

func main() {
	http.HandleFunc("GET /predict", predictHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
	go fetchLiveWeatherData()
	predict(w)
}
