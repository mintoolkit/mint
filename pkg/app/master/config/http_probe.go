package config

import (
	"strings"
	"time"
)

const (
	ProtoHTTP   = "http"
	ProtoHTTPS  = "https"
	ProtoHTTP2  = "http2"
	ProtoHTTP2C = "http2c"
	ProtoWS     = "ws"
	ProtoWSS    = "wss"
)

func IsProto(value string) bool {
	switch strings.ToLower(value) {
	case ProtoHTTP,
		ProtoHTTPS,
		ProtoHTTP2,
		ProtoHTTP2C,
		ProtoWS,
		ProtoWSS:
		return true
	default:
		return false
	}
}

// HTTPProbeCmd provides the HTTP probe parameters
type HTTPProbeCmd struct {
	Method        string   `json:"method"`
	Resource      string   `json:"resource"`
	Port          int      `json:"port"`
	Protocol      string   `json:"protocol"`
	Headers       []string `json:"headers"`
	Body          string   `json:"body"`
	BodyFile      string   `json:"body_file"`
	BodyGenerate  string   `json:"body_generate"`
	BodyIsForm    bool     `json:"body_is_form"`
	FormFieldName string   `json:"form_field_name"`
	FormFileName  string   `json:"form_file_name"`
	Username      string   `json:"username"`
	Password      string   `json:"password"`
	Crawl         bool     `json:"crawl"`

	FastCGI *FastCGIProbeWrapperConfig `json:"fastcgi,omitempty"`
}

// FastCGI permits fine-grained configuration of the fastcgi RoundTripper.
type FastCGIProbeWrapperConfig struct {
	// Root is the fastcgi root directory.
	// Defaults to the root directory of the container.
	Root string `json:"root,omitempty"`

	// The path in the URL will be split into two, with the first piece ending
	// with the value of SplitPath. The first piece will be assumed as the
	// actual resource (CGI script) name, and the second piece will be set to
	// PATH_INFO for the CGI script to use.
	SplitPath []string `json:"split_path,omitempty"`

	// Extra environment variables.
	EnvVars map[string]string `json:"env,omitempty"`

	// The duration used to set a deadline when connecting to an upstream.
	DialTimeout time.Duration `json:"dial_timeout,omitempty"`

	// The duration used to set a deadline when reading from the FastCGI server.
	ReadTimeout time.Duration `json:"read_timeout,omitempty"`

	// The duration used to set a deadline when sending to the FastCGI server.
	WriteTimeout time.Duration `json:"write_timeout,omitempty"`
}

// HTTPProbeCmds is a list of HTTPProbeCmd instances
type HTTPProbeCmds struct {
	Commands []HTTPProbeCmd `json:"commands"`
}

type HTTPProbeOptions struct {
	Do                 bool
	Full               bool
	ExitOnFailure      bool
	ExitOnFailureCount int
	FailOnStatus5xx    bool
	FailOnStatus4xx    bool

	ClientTimeout      int
	CrawlClientTimeout int
	WsClientTimeout    int //todo

	Cmds  []HTTPProbeCmd
	Ports []uint16

	StartWait        int
	RetryCount       int
	RetryWait        int
	RetryOff         bool
	ProbeConcurrency int

	CrawlMaxDepth       int
	CrawlMaxPageCount   int
	CrawlConcurrency    int
	CrawlConcurrencyMax int

	APISpecs     []string
	APISpecFiles []string

	ProxyEndpoint string
	ProxyPort     int
}
