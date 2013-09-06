package main

import (
	"code.google.com/p/go.net/websocket"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
)

var (
	Log  *log.Logger
	Conf *Config
)

func init() {
	Log = log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile)
}

func main() {
	var err error

	// parse cmd-line arguments
	flag.Parse()
	// init config
	Conf, err = InitConfig(ConfFile)
	if err != nil {
		Log.Printf("initConfig(\"%s\") failed (%s)", ConfFile, err.Error())
		return
	}
	// init log
	if Conf.Log != "" {
		f, err := os.OpenFile(Conf.Log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			Log.Printf("os.OpenFile(\"%s\", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644) failed (%s)", Conf.Log, err.Error())
			return
		}
		defer f.Close()
		Log = log.New(f, "", log.LstdFlags|log.Lshortfile)
	}
	// Set max routine
	runtime.GOMAXPROCS(Conf.MaxProcs)
	// init redis
	InitRedis()
	// init clean connected key job
	go CleanConnKeyJob()
	// for test client
	http.HandleFunc("/client", Client)
	// sub
	http.Handle("/sub", websocket.Handler(Subscribe))
	Log.Printf("gopush service start.")
	// pprof
	if Conf.Pprof == 1 {
		if Conf.PprofAddr != Conf.Addr || Conf.PprofPort != Conf.Port {
			go func() {
				profServeMux := http.NewServeMux()
				profServeMux.HandleFunc("/debug/pprof/", pprof.Index)
				profServeMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
				profServeMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
				profServeMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
				err := http.ListenAndServe(fmt.Sprintf("%s:%d", Conf.PprofAddr, Conf.PprofPort), profServeMux)
				if err != nil {
					panic(err)
				}
			}()
		}
	}
	// publish
	if Conf.PubAddr != Conf.Addr || Conf.PubPort != Conf.Port {
		go func() {
			pubServeMux := http.NewServeMux()
			pubServeMux.HandleFunc("/pub", Publish)
			err := http.ListenAndServe(fmt.Sprintf("%s:%d", Conf.PubAddr, Conf.PubPort), pubServeMux)
			if err != nil {
				panic(err)
			}
		}()
	} else {
		http.HandleFunc("/pub", Publish)
	}
	// start listen and pending here
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", Conf.Addr, Conf.Port), nil); err != nil {
		Log.Printf("http.ListenAdServe(\"%s:%d\") failed (%s)", Conf.Addr, Conf.Port, err.Error())
		return
	}

	Log.Printf("gopush service exit.")
}
