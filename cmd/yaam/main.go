// Command yaam is an MCP-first web SQL client for PostgreSQL: `yaam serve`
// is the single main process with the web UI, and `yaam mcp <connection>`
// is the stdio relay an Agent's .mcp.json runs.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wk-j/yaam/internal/config"
)

const usage = `usage:
  yaam serve [-addr 127.0.0.1:7341]   run the main process (web UI, Approval, logs)
  yaam mcp <connection>               stdio MCP relay for an Agent's .mcp.json
  yaam open                           open the web UI in the browser
  yaam install                        run yaam serve at login (launchd)
  yaam uninstall                      stop running yaam serve at login`

// errNotRunning is printed when the main process cannot be reached
// (ADR 0009: the relay never starts it).
var errNotRunning = errors.New(`yaam serve is not running; set it up once with "yaam install"`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	args := os.Args[2:]
	switch os.Args[1] {
	case "serve":
		err = cmdServe(args)
	case "mcp":
		err = cmdMCP(args)
	case "open":
		err = cmdOpen(args)
	case "install":
		err = cmdInstall(args)
	case "uninstall":
		err = cmdUninstall(args)
	case "-h", "-help", "--help", "help":
		fmt.Println(usage)
		return
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// paths of the files under YAAM_HOME.
type paths struct {
	home, socket, serveJSON, serveLog string
}

func homePaths() (paths, error) {
	home, err := config.Home()
	if err != nil {
		return paths{}, err
	}
	return paths{
		home:      home,
		socket:    filepath.Join(home, "yaam.sock"),
		serveJSON: filepath.Join(home, "serve.json"),
		serveLog:  filepath.Join(home, "serve.log"),
	}, nil
}
