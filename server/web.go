package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"shared"

	"github.com/gorilla/mux"
)

const (
	hostName = "proxy.me"
)

// Web - simple web Web
type Web struct {
	port int
	host string
}

func (server Web) start(conf *Config) {
	defer (*conf).locker.Done()

	fmt.Println("TODO: WEB SERVER")
	serve(conf)
}

func placePack(conf *Config, pack *ProxyPack) {
	(*conf).Lock()
	defer (*conf).Unlock()
	(*conf).pool[(*pack).Request.ID] = pack
}

func bindSubdomainHandler(conf *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		req, _ := shared.RequestFromRequest(r)

		vars := mux.Vars(r)
		subdomain := vars["subdomain"]

		signal := make(chan int)

		placePack(conf, &ProxyPack{
			Request: req,
			signal:  signal,
		})
		(*conf).Stats.start()

		if spaceSignal, ok := (*conf).space["test"]; ok {
			spaceSignal <- req.ID
		}

		select {
		case <-signal:

			if d, ok := (*conf).pool[req.ID]; ok {
				// fmt.Fprintf(w, "\n\n%q\n\n%q\n\n", (*d).Response.Status, (*d).Response.Body)
				resp := (*d).Response
				fmt.Printf("> [%d] %s\n", resp.Status, (*d).Request.Path)

				for _, header := range resp.Headers {
					for _, value := range header.Value {
						w.Header().Set(header.Name, value)
					}
				}

				w.WriteHeader(resp.Status)
				w.Write(resp.Body)
				(*conf).Stats.complete()
			}

		case <-time.Tick(100 * time.Second):
			fmt.Println("TIMEOUT ERROR!")
			conf.Stats.timeout()
			w.WriteHeader(http.StatusGatewayTimeout)
			fmt.Fprintf(w, "ERROR: TIMEOUT\n")
			fmt.Fprintf(w, "Method:     %q\n", r.Method)
			fmt.Fprintf(w, "RequestURI: %q\n", r.RequestURI)
			fmt.Fprintf(w, "RemoteAddr: %q\n", r.RemoteAddr)
			fmt.Fprintf(w, "SUBDOMAIN:  %q\n\n", subdomain)
		}

		// time.Sleep(500 * time.Millisecond)

		// fmt.Fprintln(w, string(dumped))
	}
}

func bindStatsHandler(conf *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, time.Now().String())
		raw, _ := json.Marshal((*conf).Stats)
		fmt.Fprintln(w, string(raw))
	}
}

func serve(conf *Config) {
	router := mux.NewRouter()

	router.Host(
		"{subdomain:.+}." + hostName,
	).HandlerFunc(
		bindSubdomainHandler(conf),
	)

	router.HandleFunc("/stats", bindStatsHandler(conf))

	router.Methods("GET").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "//"+hostName+"/stats", 302)
		},
	)

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(conf.Port), router))
}
