package main

import "net/http"

func (s *server) spaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
}
func (s *server) handleGetLayout(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }
func (s *server) handlePutLayout(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }
