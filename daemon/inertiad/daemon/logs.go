package daemon

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"

	docker "github.com/docker/docker/client"
	"github.com/ubclaunchpad/inertia/api"
	"github.com/ubclaunchpad/inertia/daemon/inertiad/containers"
	"github.com/ubclaunchpad/inertia/daemon/inertiad/log"
)

// logHandler handles requests for container logs
func (s *Server) logHandler(w http.ResponseWriter, r *http.Request) {
	var (
		stream bool
		err    error
	)

	// Get container name and stream from request query params
	params := r.URL.Query()
	container := params.Get(api.Container)
	streamParam := params.Get(api.Stream)
	if streamParam != "" {
		s, err := strconv.ParseBool(streamParam)
		if err != nil {
			println(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stream = s
	} else {
		stream = false
	}

	// Determine number of entries to fetch
	entriesParam := params.Get(api.Entries)
	var entries int
	if entriesParam != "" {
		if entries, err = strconv.Atoi(entriesParam); err != nil {
			http.Error(w, "invalid number of entries", http.StatusBadRequest)
			return
		}
	}
	if entries == 0 {
		entries = 500
	}

	// Upgrade to websocket connection if required, otherwise just set up a
	// standard logger
	var logger *log.DaemonLogger
	if stream {
		socket, err := s.websocket.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		logger = log.NewLogger(log.LoggerOptions{
			Stdout:     os.Stdout,
			Socket:     socket,
			HTTPWriter: w,
		})
	} else {
		logger = log.NewLogger(log.LoggerOptions{
			Stdout:     os.Stdout,
			HTTPWriter: w,
		})
	}

	logs, err := containers.ContainerLogs(s.docker, containers.LogOptions{
		Container: container,
		Stream:    stream,
		Entries:   entries,
	})
	if err != nil {
		if docker.IsErrNotFound(err) {
			logger.WriteErr(err.Error(), http.StatusNotFound)
		} else {
			logger.WriteErr(err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer logs.Close()

	if stream {
		var stop = make(chan struct{})
		socket, err := logger.GetSocketWriter()
		if err != nil {
			logger.WriteErr(err.Error(), http.StatusInternalServerError)
		}
		log.FlushRoutine(socket, logs, stop)
		defer logger.Close()
		defer close(stop)
	} else {
		buf := new(bytes.Buffer)
		buf.ReadFrom(logs)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, buf.String())
	}
}
