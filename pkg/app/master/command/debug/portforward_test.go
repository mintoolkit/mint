package debug

import (
	"io"
	"net"
	"testing"

	dockerapi "github.com/fsouza/go-dockerclient"
)

func TestStartPortForwarder_EchoAndClose(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo server: %v", err)
	}
	defer echoLn.Close()

	go func() {
		for {
			conn, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	_, echoPort, err := net.SplitHostPort(echoLn.Addr().String())
	if err != nil {
		t.Fatalf("split echo addr: %v", err)
	}

	bindings := map[dockerapi.Port][]dockerapi.PortBinding{
		dockerapi.Port(echoPort + "/tcp"): {
			{HostIP: "127.0.0.1", HostPort: ""},
		},
	}

	pf, err := StartPortForwarder("127.0.0.1", bindings)
	if err != nil {
		t.Fatalf("StartPortForwarder error: %v", err)
	}
	defer pf.Close()

	forwarded := pf.Bindings()
	if len(forwarded) != 1 {
		t.Fatalf("expected 1 forwarded binding, got %d", len(forwarded))
	}

	conn, err := net.Dial("tcp", forwarded[0].HostAddr)
	if err != nil {
		t.Fatalf("dial forwarded port %s: %v", forwarded[0].HostAddr, err)
	}
	defer conn.Close()

	msg := "hello-mint-debug"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("write to forwarded conn: %v", err)
	}
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
	}

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read from forwarded conn: %v", err)
	}
	if string(got) != msg {
		t.Errorf("echo mismatch: got %q, want %q", string(got), msg)
	}

	if err := pf.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
	if err := pf.Close(); err != nil {
		t.Errorf("second Close() error: %v", err)
	}
}

func TestStartPortForwarder_Errors(t *testing.T) {
	if _, err := StartPortForwarder("", nil); err == nil {
		t.Error("expected error when targetIP is empty")
	}

	_, err := StartPortForwarder("127.0.0.1", map[dockerapi.Port][]dockerapi.PortBinding{
		"53/udp": {{HostIP: "127.0.0.1", HostPort: "0"}},
	})
	if err == nil {
		t.Error("expected error for unsupported udp protocol")
	}
}

func TestResolveDockerTargetIP(t *testing.T) {
	if got := resolveDockerTargetIP(nil); got != "" {
		t.Errorf("expected empty IP for nil container, got %q", got)
	}

	c1 := &dockerapi.Container{
		NetworkSettings: &dockerapi.NetworkSettings{
			IPAddress: "172.17.0.2",
		},
	}
	if got := resolveDockerTargetIP(c1); got != "172.17.0.2" {
		t.Errorf("got %q, want 172.17.0.2", got)
	}

	c2 := &dockerapi.Container{
		NetworkSettings: &dockerapi.NetworkSettings{
			Networks: map[string]dockerapi.ContainerNetwork{
				"custom_net": {IPAddress: "172.20.0.5"},
			},
		},
	}
	if got := resolveDockerTargetIP(c2); got != "172.20.0.5" {
		t.Errorf("got %q, want 172.20.0.5", got)
	}
}
