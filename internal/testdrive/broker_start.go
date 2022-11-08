package testdrive

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/cloudfoundry/cloud-service-broker/pkg/client"
	"github.com/cloudfoundry/cloud-service-broker/utils/freeport"
	"github.com/pborman/uuid"
)

type StartBrokerOption func(config *startBrokerConfig)

type startBrokerConfig struct {
	extraEnv []string
	stdout   io.Writer
	stderr   io.Writer
}

func StartBroker(csbPath, bpk, db string, opts ...StartBrokerOption) (*Broker, error) {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
		cfg    startBrokerConfig
	)

	for _, o := range opts {
		o(&cfg)
	}

	port, err := freeport.Port()
	if err != nil {
		return nil, err
	}

	username := uuid.New()
	password := uuid.New()

	cmd := exec.Command(csbPath, "serve")
	cmd.Dir = bpk
	cmd.Env = append(
		os.Environ(),
		"CSB_LISTENER_HOST=localhost",
		"DB_TYPE=sqlite3",
		fmt.Sprintf("DB_PATH=%s", db),
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("SECURITY_USER_NAME=%s", username),
		fmt.Sprintf("SECURITY_USER_PASSWORD=%s", password),
	)
	cmd.Env = append(cmd.Env, cfg.extraEnv...)

	switch cfg.stdout {
	case nil:
		cmd.Stdout = &stdout
	default:
		cmd.Stdout = io.MultiWriter(&stdout, cfg.stdout)
	}

	switch cfg.stderr {
	case nil:
		cmd.Stderr = &stderr
	default:
		cmd.Stderr = io.MultiWriter(&stderr, cfg.stderr)
	}

	clnt, err := client.New(username, password, "localhost", port)
	if err != nil {
		return nil, err
	}

	broker := Broker{
		Database: db,
		Port:     port,
		Client:   clnt,
		username: username,
		password: password,
		runner:   runCommand(cmd),
		Stdout:   &stdout,
		Stderr:   &stderr,
	}

	start := time.Now()
	for {
		response, err := http.Head(fmt.Sprintf("http://localhost:%d", port))
		switch {
		case err == nil && response.StatusCode == http.StatusOK:
			return &broker, nil
		case time.Since(start) > time.Minute:
			if err := broker.runner.stop(); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("timed out after %s waiting for broker to start: %s\n%s", time.Since(start), stdout.String(), stderr.String())
		case broker.runner.exited:
			return nil, fmt.Errorf("failed to start broker: %w\n%s\n%s", broker.runner.err, stdout.String(), stderr.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func WithEnv(extraEnv ...string) StartBrokerOption {
	return func(cfg *startBrokerConfig) {
		cfg.extraEnv = append(cfg.extraEnv, extraEnv...)
	}
}
