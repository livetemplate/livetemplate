package commands

import (
	"fmt"
	"strconv"

	"github.com/livefir/livetemplate/cmd/lvt/internal/serve"
)

func Serve(args []string) error {
	config := serve.DefaultConfig()

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 >= len(args) {
				return fmt.Errorf("--port requires a value")
			}
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return fmt.Errorf("invalid port number: %s", args[i+1])
			}
			config.Port = port
			i++

		case "--host", "-h":
			if i+1 >= len(args) {
				return fmt.Errorf("--host requires a value")
			}
			config.Host = args[i+1]
			i++

		case "--dir", "-d":
			if i+1 >= len(args) {
				return fmt.Errorf("--dir requires a value")
			}
			config.Dir = args[i+1]
			i++

		case "--mode", "-m":
			if i+1 >= len(args) {
				return fmt.Errorf("--mode requires a value")
			}
			mode := serve.ServeMode(args[i+1])
			switch mode {
			case serve.ModeComponent, serve.ModeKit, serve.ModeApp:
				config.Mode = mode
				config.AutoDetect = false
			default:
				return fmt.Errorf("invalid mode: %s (valid: component, kit, app)", args[i+1])
			}
			i++

		case "--no-browser":
			config.OpenBrowser = false

		case "--no-reload":
			config.LiveReload = false

		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	server, err := serve.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}
