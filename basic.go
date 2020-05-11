package main

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func handler(w http.ResponseWriter, r *http.Request) {
	filename := "/Users/rosaline/Documents/CMU/20Spring/18845/18845-GP/FlashServer-GP/content" + r.URL.String()
	file, _ := os.Open(filename)
	stat, _ := os.Stat(filename)
	var contentType string
	if strings.Contains(filename, ".html") ||
		strings.Contains(filename, ".htm") {
		contentType = "text/html"
	} else if strings.Contains(filename, ".gif") {
		contentType = "image/gif"
	} else if strings.Contains(filename, ".png") {
		contentType = "image/png"
	} else if strings.Contains(filename, ".jpg") ||
		strings.Contains(filename, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.Contains(filename, ".txt") {
		contentType = "text/plain"
	} else if strings.Contains(filename, ".css") {
		contentType = "text/css"
	} else if strings.Contains(filename, ".js") {
		contentType = "application/javascript"
	} else if strings.Contains(filename, ".mp4") {
		contentType = "video/mp4"
	} else if strings.Contains(filename, ".webm") {
		contentType = "video/webm"
	} else if strings.Contains(filename, ".ogg") {
		contentType = "video/ogg"
	} else if strings.Contains(filename, ".pdf") {
		contentType = "application/pdf"
	} else {
		contentType = "application/octet-stream"
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	modified := stat.ModTime().Format(http.TimeFormat)
	size := strconv.FormatInt(stat.Size(), 10)
	w.Header().Add("Server", "Web Server")
	w.Header().Add("Date", date)
	w.Header().Add("Last-Modified", modified)
	w.Header().Add("Content-Length", size)
	w.Header().Add("Content-Type", contentType)
	w.WriteHeader(200)
	io.Copy(w, file)
	file.Close()
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe("localhost:8080", nil)
}
