package debug

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

// ForwardedPort describes an active host-to-container port forwarding rule.
type ForwardedPort struct {
	HostAddr      string
	ContainerPort string
	TargetAddr    string
}

// PortForwarder proxies TCP connections from host listeners to a debugged container's network address.
type PortForwarder struct {
	mu        sync.Mutex
	listeners []net.Listener
	bindings  []ForwardedPort
	closed    bool
	wg        sync.WaitGroup
}

// StartPortForwarder opens host TCP listeners for each requested port binding and forwards traffic to targetIP.
func StartPortForwarder(targetIP string, portBindings map[dockerapi.Port][]dockerapi.PortBinding) (*PortForwarder, error) {
	if targetIP == "" {
		return nil, errors.New("cannot forward ports: target container has no IP address")
	}

	pf := &PortForwarder{}
	for cport, bindings := range portBindings {
		proto := cport.Proto()
		if proto != "" && proto != "tcp" {
			_ = pf.Close()
			return nil, fmt.Errorf("unsupported port protocol %q for %s (only tcp is supported)", proto, cport)
		}

		portNum := cport.Port()
		targetAddr := net.JoinHostPort(targetIP, portNum)

		if len(bindings) == 0 {
			bindings = []dockerapi.PortBinding{{HostIP: "127.0.0.1", HostPort: portNum}}
		}

		for _, b := range bindings {
			hostIP := b.HostIP
			if hostIP == "" {
				hostIP = "127.0.0.1"
			}
			hostPort := b.HostPort
			if hostPort == "" {
				hostPort = "0"
			}

			ln, err := net.Listen("tcp", net.JoinHostPort(hostIP, hostPort))
			if err != nil {
				_ = pf.Close()
				return nil, fmt.Errorf("listen %s:%s -> %s: %w", hostIP, hostPort, targetAddr, err)
			}

			pf.listeners = append(pf.listeners, ln)
			pf.bindings = append(pf.bindings, ForwardedPort{
				HostAddr:      ln.Addr().String(),
				ContainerPort: string(cport),
				TargetAddr:    targetAddr,
			})

			pf.wg.Add(1)
			go pf.serve(ln, targetAddr)
		}
	}

	return pf, nil
}

// Bindings returns a copy of the active host-to-container port mappings.
func (pf *PortForwarder) Bindings() []ForwardedPort {
	out := make([]ForwardedPort, len(pf.bindings))
	copy(out, pf.bindings)
	return out
}

// Close stops all listeners and waits for accept loops to exit.
func (pf *PortForwarder) Close() error {
	pf.mu.Lock()
	if pf.closed {
		pf.mu.Unlock()
		return nil
	}
	pf.closed = true
	listeners := pf.listeners
	pf.mu.Unlock()

	var firstErr error
	for _, ln := range listeners {
		if err := ln.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	pf.wg.Wait()
	return firstErr
}

func (pf *PortForwarder) serve(ln net.Listener, targetAddr string) {
	defer pf.wg.Done()
	for {
		clientConn, err := ln.Accept()
		if err != nil {
			pf.mu.Lock()
			closed := pf.closed
			pf.mu.Unlock()
			if !closed {
				log.WithError(err).Debug("debug.portforward: accept error")
			}
			return
		}

		go proxyConn(clientConn, targetAddr)
	}
}

func proxyConn(clientConn net.Conn, targetAddr string) {
	defer clientConn.Close()

	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.WithError(err).Debugf("debug.portforward: dial %s failed", targetAddr)
		return
	}
	defer targetConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(targetConn, clientConn)
		if tc, ok := targetConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, targetConn)
		if cc, ok := clientConn.(*net.TCPConn); ok {
			_ = cc.CloseWrite()
		}
	}()
	wg.Wait()
}

func resolveDockerTargetIP(info *dockerapi.Container) string {
	if info == nil || info.NetworkSettings == nil {
		return ""
	}
	if info.NetworkSettings.IPAddress != "" {
		return info.NetworkSettings.IPAddress
	}
	for _, netCfg := range info.NetworkSettings.Networks {
		if netCfg.IPAddress != "" {
			return netCfg.IPAddress
		}
	}
	return ""
}
