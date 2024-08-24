package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"

	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/app/master/probe/http/internal"
)

const defaultClientTimeout = 30

func getHTTP1Client(clientTimeout int) *http.Client {
	if clientTimeout <= 0 {
		clientTimeout = defaultClientTimeout
	}

	client := &http.Client{
		Timeout: time.Second * time.Duration(clientTimeout),
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: time.Duration(clientTimeout) * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return client
}

func getHTTP2Client(clientTimeout int, h2c bool) *http.Client {
	if clientTimeout <= 0 {
		clientTimeout = defaultClientTimeout
	}

	transport := &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Timeout:   time.Second * time.Duration(clientTimeout),
		Transport: transport,
	}

	if h2c {
		transport.AllowHTTP = true
		transport.DialTLS = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		}
	}

	return client
}

func getHTTPClient(proto string, clientTimeout int) (*http.Client, error) {
	switch proto {
	case config.ProtoHTTP2:
		return getHTTP2Client(clientTimeout, false), nil
	case config.ProtoHTTP2C:
		return getHTTP2Client(clientTimeout, true), nil
	default:
		return getHTTP1Client(clientTimeout), nil
	}

	return nil, fmt.Errorf("unsupported HTTP-family protocol %s", proto)
}

func getHTTPAddr(proto, targetHost, port string) string {
	scheme := getHTTPScheme(proto)
	return fmt.Sprintf("%s://%s:%s", scheme, targetHost, port)
}

func getHTTPScheme(proto string) string {
	var scheme string
	switch proto {
	case config.ProtoHTTP:
		scheme = proto
	case config.ProtoHTTPS:
		scheme = proto
	case config.ProtoHTTP2:
		scheme = config.ProtoHTTPS
	case config.ProtoHTTP2C:
		scheme = config.ProtoHTTP
	}

	return scheme
}

func getFastCGIClient(clientTimeout int, cfg *config.FastCGIProbeWrapperConfig) *http.Client {
	if clientTimeout <= 0 {
		clientTimeout = defaultClientTimeout
	}

	genericTimeout := time.Second * time.Duration(clientTimeout)
	var dialTimeout, readTimeout, writeTimeout time.Duration
	if dialTimeout = cfg.DialTimeout; dialTimeout == 0 {
		dialTimeout = genericTimeout
	}
	if readTimeout = cfg.ReadTimeout; readTimeout == 0 {
		readTimeout = genericTimeout
	}
	if writeTimeout = cfg.WriteTimeout; writeTimeout == 0 {
		writeTimeout = genericTimeout
	}

	return &http.Client{
		Timeout: genericTimeout,
		Transport: &internal.FastCGITransport{
			Root:         cfg.Root,
			SplitPath:    cfg.SplitPath,
			EnvVars:      cfg.EnvVars,
			DialTimeout:  dialTimeout,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}
}
