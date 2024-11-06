package main

import (
	"net/http"
	"net/http/pprof"
)

func enablePPROF() {
	prefix := "GET "
	http.HandleFunc(prefix+"/debug/pprof/", pprof.Index)
	http.HandleFunc(prefix+"/debug/pprof/cmdline", pprof.Cmdline)
	http.HandleFunc(prefix+"/debug/pprof/profile", pprof.Profile)
	http.HandleFunc(prefix+"/debug/pprof/symbol", pprof.Symbol)
	http.HandleFunc(prefix+"/debug/pprof/trace", pprof.Trace)
}
