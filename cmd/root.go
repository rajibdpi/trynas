// Package cmd owns standalone command execution and server startup.
package cmd

import "github.com/rajibdpi/trynas/config"

// Execute loads command configuration and runs the standalone server.
func Execute() error {
	if err := config.Load(); err != nil {
		return err
	}
	return serve()
}
