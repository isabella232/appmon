package track

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"time"
)

// ClientConfig contains configuration settings that the client requires to send
// tracking information to the server.
type ClientConfig struct {
	// NewViewURL is the URL that the client POSTs new views to.
	NewViewURL string
}

// ClientData contains client-specific information that the client requires to
// send tracking information to the server.
type ClientData struct {
	// Win is the value that should be associated with all views and
	// calls originating from this window.
	Win int
}

func MakeClientConfig(rt *mux.Router) (config *ClientConfig, err error) {
	newViewURL, err := rt.GetRoute(trackCreateView).URL("win", ":win")
	if err != nil {
		return
	}
	return &ClientConfig{
		NewViewURL: newViewURL.String(),
	}, nil
}

// CurrentUser, if set, is called to determine the currently authenticated user
// for the current request. The returned user ID is stored in the View record if
// nonempty. If err != nil, an HTTP 500 is returned.
var CurrentUser func(r *http.Request) (user string, err error)

const (
	trackQueryCalls = "track:queryCalls"

	trackCreateView = "track:createView"
	trackQueryViews = "track:queryViews"
)

func APIRouter(rt *mux.Router) *mux.Router {
	wins := rt.PathPrefix("/wins").Subrouter()
	win := wins.PathPrefix("/{win}").Subrouter()

	win.Path("/views").Methods("POST").Handler(storeClientID(http.HandlerFunc(createView))).Name(trackCreateView)
	win.Path("/views").Methods("GET").HandlerFunc(queryViews).Name(trackQueryViews)

	view := win.PathPrefix("/views/{seq}").Subrouter()

	view.Path("/calls").Methods("GET").HandlerFunc(queryCalls).Name(trackQueryCalls)
	return rt
}

func createView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	win, err := strconv.Atoi(vars["win"])
	if err != nil || win <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var v *View
	err = json.NewDecoder(r.Body).Decode(&v)
	if err != nil {
		log.Printf("Decode: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if v.Win != win {
		log.Printf("View.Win (%d) != win route parameter (%d)", v.Win, win)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v.Date = time.Now()
	v.Client = Client{ClientID: GetClientID(r)}
	if CurrentUser != nil {
		user, err := CurrentUser(r)
		if err != nil {
			log.Printf("CurrentUser failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if user != "" {
			v.Client.User = NullString{user, true}
		}
	}

	err = InsertView(v)
	if err != nil {
		log.Printf("InsertView: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func queryViews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	win, err := strconv.Atoi(vars["win"])
	if err != nil || win <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	views, err := QueryViews("WHERE win = $1", win)
	if err != nil {
		log.Printf("QueryViews: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if views == nil {
		views = []*View{}
	}
	json.NewEncoder(w).Encode(views)
}

func queryCalls(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	win, err := strconv.Atoi(vars["win"])
	if err != nil || win <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	seq, err := strconv.Atoi(vars["seq"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	calls, err := QueryCalls("WHERE view_win = $1 AND view_seq = $2", win, seq)
	if err != nil {
		log.Printf("QueryCalls: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if calls == nil {
		calls = []*Call{}
	}
	json.NewEncoder(w).Encode(calls)
}