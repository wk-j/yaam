package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/wk-j/yaam/internal/app"
)

// cmdMCP is the stdio relay (ADR 0006): it hands the Agent's MCP stream to
// the main process, which runs the MCP server for this Connection.
func cmdMCP(args []string) error {
	if len(args) != 1 || args[0] == "" {
		return errors.New("usage: yaam mcp <connection>")
	}
	p, err := homePaths()
	if err != nil {
		return err
	}
	conn, err := net.Dial("unix", p.socket)
	if err != nil {
		return errNotRunning
	}
	defer conn.Close()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	hello, _ := json.Marshal(app.Hello{Connection: args[0], Cwd: cwd})
	if _, err := conn.Write(append(hello, '\n')); err != nil {
		return errNotRunning
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return errNotRunning
	}
	var reply app.HelloReply
	if err := json.Unmarshal(line, &reply); err != nil {
		return errors.New("unexpected reply from yaam serve")
	}
	if !reply.OK {
		return errors.New(reply.Error)
	}

	done := make(chan struct{})
	go func() {
		io.Copy(os.Stdout, br)
		close(done)
	}()
	go func() {
		io.Copy(conn, os.Stdin)
		// The Agent closed stdin: tell the main process, which cancels
		// anything still waiting and closes the socket.
		conn.(*net.UnixConn).CloseWrite()
	}()
	<-done
	return nil
}

// cmdOpen opens the web UI of the running yaam serve with this run's token.
func cmdOpen(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: yaam open")
	}
	p, err := homePaths()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p.serveJSON)
	if errors.Is(err, os.ErrNotExist) {
		return errNotRunning
	}
	if err != nil {
		return err
	}
	var info serveInfo
	if err := json.Unmarshal(data, &info); err != nil || info.URL == "" {
		return errors.New("serve.json is unreadable; restart yaam serve")
	}
	return exec.Command("/usr/bin/open", info.URL+"/#token="+info.Token).Run()
}
