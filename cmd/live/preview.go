package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func localPreviewProxy(upstream *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ModifyResponse = func(response *http.Response) error {
		location := response.Header.Get("Location")
		if location == "" {
			return nil
		}
		target, err := url.Parse(location)
		if err != nil {
			return err
		}
		if target.Host != "" && target.Host != upstream.Host {
			return nil
		}
		if target.Path != upstream.Path && !strings.HasPrefix(target.Path, upstream.Path+"/") {
			return nil
		}
		target.Scheme, target.Host, target.RawPath = "", "", ""
		target.Path = "/live-preview" + strings.TrimPrefix(target.Path, upstream.Path)
		response.Header.Set("Location", target.String())
		return nil
	}
	return proxy
}
