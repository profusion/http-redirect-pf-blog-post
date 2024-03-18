package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

var from int
var to string

func init() {
	flag.IntVar(&from, "from", 5555, "Local port to get requests")
	flag.StringVar(&to, "to", "", "Target server to redirect request to")
}

func main() {
	flag.Parse()

	Listen()
}

type proxy struct{}

func Listen() {
	p := &proxy{}
	srvr := http.Server{
		Addr:    fmt.Sprintf(":%d", from),
		Handler: p,
	}
	if err := srvr.ListenAndServe(); err != nil {
		slog.Error("Server is down", "Error", err)
	}
}

// ServeHTTP implements http.Handler.
func (p *proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	PreRequestHook(req)

	// Remove original URL for redirect
	req.RequestURI = ""

	// Set URL accordingly
	req.URL.Host = to
	if req.TLS == nil {
		req.URL.Scheme = "http"
	} else {
		req.URL.Scheme = "https"
	}

	// Remove connection headers
	// (will be replaced by redirect client)
	DropHopHeaders(&req.Header)

	// Register Proxy Request
	SetProxyHeader(req)

	// Resend request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(rw, "Server Error: Redirect failed", http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	// Once again, remove connection headers
	DropHopHeaders(&resp.Header)

	// Prepare and send response
	CopyHeaders(rw.Header(), &resp.Header)
	rw.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(rw, resp.Body); err != nil {
		slog.Error("Error writing response", "error", err)
	}
}

func CopyHeaders(src http.Header, dst *http.Header) {
	for headingName, headingValues := range src {
		for _, value := range headingValues {
			dst.Add(headingName, value)
		}
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func DropHopHeaders(head *http.Header) {
	for _, header := range hopHeaders {
		head.Del(header)
	}
}

func SetProxyHeader(req *http.Request) {
	headerName := "X-Forwarded-for"
	target := to
	if prior, ok := req.Header[headerName]; ok {
		// Not first proxy, append
		target = strings.Join(prior, ", ") + ", " + target
	}
	req.Header.Set(headerName, target)
}
