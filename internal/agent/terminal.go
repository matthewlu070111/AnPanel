package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	"github.com/gorilla/websocket"
)

var agentUpgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func (s *server) terminal(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !safeID.MatchString(id) {
		http.Error(w, "invalid container id", 400)
		return
	}
	cmd := exec.CommandContext(r.Context(), "docker", "exec", "-i", id, "/bin/sh")
	if _, err := exec.LookPath("script"); err == nil {
		cmd = exec.CommandContext(r.Context(), "script", "-qefc", "docker exec -it "+id+" /bin/sh", "/dev/null")
	}
	streamTerminal(w, r, cmd)
}

func (s *server) hostTerminal(w http.ResponseWriter, r *http.Request) {
	if os.Geteuid() != 0 {
		http.Error(w, "host terminal requires the agent to run as root", http.StatusForbidden)
		return
	}
	cmd := exec.CommandContext(r.Context(), "/bin/bash", "-l")
	if _, err := exec.LookPath("script"); err == nil {
		cmd = exec.CommandContext(r.Context(), "script", "-qefc", "/bin/bash -l", "/dev/null")
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	streamTerminal(w, r, cmd)
}

func streamTerminal(w http.ResponseWriter, r *http.Request, cmd *exec.Cmd) {
	conn, err := agentUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				if e := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); e != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if _, err = stdin.Write(msg); err != nil {
			break
		}
	}
	_ = stdin.Close()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	_ = writer.Close()
	<-done
}

func (c *Client) DialTerminal(ctx context.Context, id string) (*websocket.Conn, error) {
	return c.dialTerminal(ctx, "/v1/docker/terminal?id="+url.QueryEscape(id))
}

func (c *Client) DialHostTerminal(ctx context.Context) (*websocket.Conn, error) {
	return c.dialTerminal(ctx, "/v1/host/terminal")
}

func (c *Client) dialTerminal(ctx context.Context, path string) (*websocket.Conn, error) {
	d := websocket.Dialer{NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.token)
	conn, _, err := d.DialContext(ctx, "ws://unix"+path, h)
	return conn, err
}
