package observability

import (
	"net/http"
)

// headerCarrier adapts http.Header to the TextMapCarrier interface
type headerCarrier http.Header

// Get returns the value associated with the passed key.
func (hc headerCarrier) Get(key string) string {
	values := http.Header(hc).Get(key)
	return values
}

// Set stores the key-value pair.
func (hc headerCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

// Keys lists the keys stored in this carrier.
func (hc headerCarrier) Keys() []string {
	keys := make([]string, 0, len(hc))
	for k := range http.Header(hc) {
		keys = append(keys, k)
	}
	return keys
}
