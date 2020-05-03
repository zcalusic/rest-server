package restserver

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/restic/rest-server/quota"
)

func (s *Server) debugHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s", r.Method, r.URL)
			next.ServeHTTP(w, r)
		})
}

func (s *Server) logHandler(next http.Handler) http.Handler {
	accessLog, err := os.OpenFile(s.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	return handlers.CombinedLoggingHandler(accessLog, next)
}

func (s *Server) checkAuth(r *http.Request) (username string, ok bool) {
	if s.NoAuth {
		return username, true
	}
	var password string
	username, password, ok = r.BasicAuth()
	if !ok || !s.htpasswdFile.Validate(username, password) {
		return "", false
	}
	return username, true
}

// NewHandler returns the master HTTP multiplexer/router.
func NewHandler(server *Server) (http.Handler, error) {
	if !server.NoAuth {
		var err error
		server.htpasswdFile, err = NewHtpasswdFromFile(filepath.Join(server.Path, ".htpasswd"))
		if err != nil {
			return nil, fmt.Errorf("cannot load .htpasswd (use --no-auth to disable): %v", err)
		}
	}

	if server.MaxRepoSize > 0 {
		log.Printf("Initializing quota (can take a while)...")
		qm, err := quota.New(server.Path, server.MaxRepoSize)
		if err != nil {
			return nil, err
		}
		server.quotaManager = qm
		log.Printf("Quota initialized, currenly using %.2f GiB", float64(qm.SpaceUsed()/1024/1024))
	}

	mux := http.NewServeMux()
	if server.Prometheus {
		// FIXME: need auth like in previous version?
		mux.Handle("/metrics", promhttp.Handler())
	}
	mux.Handle("/", server)

	var handler http.Handler = mux
	if server.Debug {
		handler = server.debugHandler(handler)
	}
	if server.Log != "" {
		handler = server.logHandler(handler)
	}
	return handler, nil
}
