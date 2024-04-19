package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(responseWriter http.ResponseWriter, request *http.Request) {
		// if no valid paramters, then render index page
		http.ServeFile(responseWriter, request, "../public/index.html")

		// if it has id, then fetch record by id from database

		// if the record is a short url, then redirect to the long url

		// else, render view message page

	})
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}