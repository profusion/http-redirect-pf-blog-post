package protocol

import "net/http"

type HttpRedirectPlugin interface {
	PreRequestHook(*http.Request)
}
