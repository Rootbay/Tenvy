package agent

import (
	"net/http"
	"strings"
)

func applyRequestDecorations(req *http.Request, headers []CustomHeader, cookies []CustomCookie) {
	if req == nil {
		return
	}
	if req.Header == nil {
		req.Header = http.Header{}
	}
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" || value == "" {
			continue
		}
		req.Header.Add(key, value)
	}
	for _, cookie := range cookies {
		name := strings.TrimSpace(cookie.Name)
		value := strings.TrimSpace(cookie.Value)
		if name == "" || value == "" {
			continue
		}
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
}

func applyHeaderMapDecorations(dst http.Header, headers []CustomHeader, cookies []CustomCookie) {
	if dst == nil {
		return
	}
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" || value == "" {
			continue
		}
		dst.Add(key, value)
	}
	for _, cookie := range cookies {
		name := strings.TrimSpace(cookie.Name)
		value := strings.TrimSpace(cookie.Value)
		if name == "" || value == "" {
			continue
		}
		httpCookie := (&http.Cookie{Name: name, Value: value}).String()
		if httpCookie == "" {
			continue
		}
		dst.Add("Cookie", httpCookie)
	}
}
