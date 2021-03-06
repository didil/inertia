package daemon

import (
	"net/http"
	"os"

	"github.com/ubclaunchpad/inertia/daemon/inertiad/containers"
	"github.com/ubclaunchpad/inertia/daemon/inertiad/log"
)

const (
	msgNoDeployment = "No deployment is currently active on this remote - try running 'inertia [remote] up'"
)

// downHandler tries to take the deployment offline
func (s *Server) downHandler(w http.ResponseWriter, r *http.Request) {
	if status, _ := s.deployment.GetStatus(s.docker); len(status.Containers) == 0 {
		http.Error(w, msgNoDeployment, http.StatusPreconditionFailed)
		return
	}

	logger := log.NewLogger(log.LoggerOptions{
		Stdout:     os.Stdout,
		HTTPWriter: w,
	})
	defer logger.Close()

	if err := s.deployment.Down(s.docker, logger); err == containers.ErrNoContainers {
		logger.WriteErr(err.Error(), http.StatusPreconditionFailed)
		return
	} else if err != nil {
		logger.WriteErr(err.Error(), http.StatusInternalServerError)
		return
	}

	logger.WriteSuccess("Project shut down.", http.StatusOK)
}
