package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
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
	conn, err := agentUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", id, "/bin/sh")
	if _, err := exec.LookPath("script"); err == nil {
		cmd = exec.CommandContext(ctx, "script", "-qefc", "docker exec -it "+id+" /bin/sh", "/dev/null")
	}
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
	cancel()
	_ = cmd.Wait()
	_ = writer.Close()
	<-done
}

func (c *Client) DialTerminal(ctx context.Context, id string) (*websocket.Conn, error) {
	d := websocket.Dialer{NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	}}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.token)
	conn, _, err := d.DialContext(ctx, "ws://unix/v1/docker/terminal?id="+url.QueryEscape(id), h)
	return conn, err
}
