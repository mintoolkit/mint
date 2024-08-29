package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/app/master/inspectors/container"
	"github.com/mintoolkit/mint/pkg/app/master/inspectors/pod"
	"github.com/mintoolkit/mint/pkg/app/master/probe/data"
)

const (
	defaultProbeRetryCount = 5

	defaultHTTPPortStr    = "80"
	defaultHTTPSPortStr   = "443"
	defaultFastCGIPortStr = "9000"

	defaultFormFieldName = "file"
	defaultFormFileName  = "file.data"
)

const (
	HeaderContentType = "Content-Type"
)

var (
	ErrFailOnStatus5xx = errors.New("failed on status code 5xx")
)

type ovars = app.OutVars

// CustomProbe is a custom HTTP probe
type CustomProbe struct {
	xc *app.ExecutionContext

	opts config.HTTPProbeOptions

	ports              []string
	targetHost         string
	ipcMode            string
	availableHostPorts map[string]string

	APISpecProbes []apiSpecInfo

	printState bool

	CallCount uint64
	ErrCount  uint64
	OkCount   uint64

	doneChan           chan struct{}
	workers            sync.WaitGroup
	concurrentCrawlers chan struct{}
}

// NewEndpointProbe creates a new custom HTTP probe for an endpoint
func NewEndpointProbe(
	xc *app.ExecutionContext,
	targetEndpoint string,
	ports []uint,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc, targetEndpoint, "", opts, printState)
	if len(ports) == 0 {
		ports = []uint{80}
	}

	for _, pnum := range ports {
		probe.ports = append(probe.ports, fmt.Sprintf("%d", pnum))
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

// NewContainerProbe creates a new custom HTTP probe
func NewContainerProbe(
	xc *app.ExecutionContext,
	inspector *container.Inspector,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc,
		inspector.TargetHost,
		inspector.SensorIPCMode,
		opts,
		printState)

	for nsPortKey, nsPortData := range inspector.AvailablePorts {
		log.Debugf("HTTP probe - target's network port key='%s' data='%#v'", nsPortKey, nsPortData)

		if nsPortKey.Proto() != "tcp" {
			log.Debugf("HTTP probe - skipping non-tcp port => %v", nsPortKey)
			continue
		}

		if nsPortData.HostPort == "" {
			log.Debugf("HTTP probe - skipping network setting without port data => %v", nsPortKey)
			continue
		}

		probe.availableHostPorts[nsPortData.HostPort] = nsPortKey.Port()
	}

	log.Debugf("HTTP probe - available host ports => %+v", probe.availableHostPorts)

	tmpAvailableHostPorts := map[string]string{}
	for k, v := range probe.availableHostPorts {
		tmpAvailableHostPorts[k] = v
	}
	if len(probe.opts.Ports) > 0 {
		for _, pnum := range probe.opts.Ports {
			pspec := dockerapi.Port(fmt.Sprintf("%v/tcp", pnum))
			if _, ok := inspector.AvailablePorts[pspec]; ok {
				if inspector.SensorIPCMode == container.SensorIPCModeDirect {
					probe.ports = append(probe.ports, fmt.Sprintf("%d", pnum))
				} else {
					probe.ports = append(probe.ports, inspector.AvailablePorts[pspec].HostPort)
				}
			} else {
				log.Debugf("HTTP probe - ignoring port => %v", pspec)
			}
		}
		log.Debugf("HTTP probe - filtered ports => %+v", probe.ports)
	} else {
		//order the port list based on the order of the 'EXPOSE' instructions
		if len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) > 0 {
			for epi := len(inspector.ImageInspector.DockerfileInfo.ExposedPorts) - 1; epi >= 0; epi-- {
				portInfo := inspector.ImageInspector.DockerfileInfo.ExposedPorts[epi]
				if !strings.Contains(portInfo, "/") {
					portInfo = fmt.Sprintf("%v/tcp", portInfo)
				}

				pspec := dockerapi.Port(portInfo)

				if _, ok := inspector.AvailablePorts[pspec]; ok {
					hostPort := inspector.AvailablePorts[pspec].HostPort
					if inspector.SensorIPCMode == container.SensorIPCModeDirect {
						if containerPort := tmpAvailableHostPorts[hostPort]; containerPort != "" {
							probe.ports = append(probe.ports, containerPort)
						} else {
							log.Debugf("HTTP probe - could not find container port from host port => %v", hostPort)
						}
					} else {
						probe.ports = append(probe.ports, hostPort)
					}

					if _, ok := tmpAvailableHostPorts[hostPort]; ok {
						log.Debugf("HTTP probe - delete exposed port from the available host ports => %v (%v)", hostPort, portInfo)
						//remove the port, so we can asign the rest of them in the last loop
						delete(tmpAvailableHostPorts, hostPort)
					}
				} else {
					log.Debugf("HTTP probe - Unknown exposed port - %v", portInfo)
				}
			}
		}

		for hostPort, containerPort := range tmpAvailableHostPorts {
			if inspector.SensorIPCMode == container.SensorIPCModeDirect {
				probe.ports = append(probe.ports, containerPort)
			} else {
				probe.ports = append(probe.ports, hostPort)
			}
		}

		log.Debugf("HTTP probe - probe.Ports => %+v", probe.ports)
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

func NewPodProbe(
	xc *app.ExecutionContext,
	inspector *pod.Inspector,
	opts config.HTTPProbeOptions,
	printState bool,
) (*CustomProbe, error) {
	probe := newCustomProbe(xc, inspector.TargetHost(), "", opts, printState)

	for nsPortKey, nsPortData := range inspector.AvailablePorts() {
		log.Debugf("HTTP probe - target's network port key='%s' data='%#v'", nsPortKey, nsPortData)
		probe.availableHostPorts[nsPortData.HostPort] = nsPortKey.Port()
	}

	log.Debugf("HTTP probe - available host ports => %+v", probe.availableHostPorts)

	if len(probe.opts.Ports) > 0 {
		for _, pnum := range probe.opts.Ports {
			pspec := dockerapi.Port(fmt.Sprintf("%v/tcp", pnum))
			if port, ok := inspector.AvailablePorts()[pspec]; ok {
				probe.ports = append(probe.ports, port.HostPort)
			} else {
				log.Debugf("HTTP probe - ignoring port => %v", pspec)
			}
		}

		log.Debugf("HTTP probe - filtered ports => %+v", probe.ports)
	} else {
		for hostPort := range probe.availableHostPorts {
			probe.ports = append(probe.ports, hostPort)
		}

		log.Debugf("HTTP probe - probe.Ports => %+v", probe.ports)
	}

	if len(probe.opts.APISpecFiles) > 0 {
		probe.loadAPISpecFiles()
	}

	return probe, nil
}

func newCustomProbe(
	xc *app.ExecutionContext,
	targetHost string,
	ipcMode string,
	opts config.HTTPProbeOptions,
	printState bool,
) *CustomProbe {
	//note: the default probe should already be there if the user asked for it

	//-1 means disabled
	if opts.CrawlMaxDepth == 0 {
		opts.CrawlMaxDepth = defaultCrawlMaxDepth
	}

	//-1 means disabled
	if opts.CrawlMaxPageCount == 0 {
		opts.CrawlMaxPageCount = defaultCrawlMaxPageCount
	}

	//-1 means disabled
	if opts.CrawlConcurrency == 0 {
		opts.CrawlConcurrency = defaultCrawlConcurrency
	}

	//-1 means disabled
	if opts.CrawlConcurrencyMax == 0 {
		opts.CrawlConcurrencyMax = defaultMaxConcurrentCrawlers
	}

	probe := &CustomProbe{
		xc:                 xc,
		opts:               opts,
		printState:         printState,
		targetHost:         targetHost,
		ipcMode:            ipcMode,
		availableHostPorts: map[string]string{},
		doneChan:           make(chan struct{}),
	}

	if opts.CrawlConcurrencyMax > 0 {
		probe.concurrentCrawlers = make(chan struct{}, opts.CrawlConcurrencyMax)
	}

	return probe
}

func (p *CustomProbe) Ports() []string {
	return p.ports
}

func (ref *CustomProbe) retryCount() int {
	if ref.opts.RetryOff || (ref.opts.RetryCount == 0) {
		return 0
	}

	result := defaultProbeRetryCount
	if ref.opts.RetryCount > -1 {
		result = ref.opts.RetryCount
	}

	return result
}

// Start starts the HTTP probe instance execution
func (p *CustomProbe) Start() {
	if p.printState {
		p.xc.Out.State("http.probe.starting",
			ovars{
				"message": "WAIT FOR HTTP PROBE TO FINISH",
			})
	}

	go func() {
		//TODO: need to do a better job figuring out if the target app is ready to accept connections
		time.Sleep(9 * time.Second) //base start wait time
		if p.opts.StartWait > 0 {
			if p.printState {
				p.xc.Out.State("http.probe.start.wait", ovars{"time": p.opts.StartWait})
			}

			//additional wait time
			time.Sleep(time.Duration(p.opts.StartWait) * time.Second)

			if p.printState {
				p.xc.Out.State("http.probe.start.wait.done")
			}
		}

		if p.printState {
			p.xc.Out.State("http.probe.running")
		}

		log.Info("HTTP probe started...")

		findIdx := func(ports []string, target string) int {
			for idx, val := range ports {
				if val == target {
					return idx
				}
			}
			return -1
		}

		httpIdx := findIdx(p.ports, defaultHTTPPortStr)
		httpsIdx := findIdx(p.ports, defaultHTTPSPortStr)
		if httpIdx != -1 && httpsIdx != -1 && httpsIdx < httpIdx {
			//want to probe http first
			log.Debugf("http.probe - swapping http and https ports (http=%v <-> https=%v)",
				httpIdx, httpsIdx)

			p.ports[httpIdx], p.ports[httpsIdx] = p.ports[httpsIdx], p.ports[httpIdx]
		}

		if p.printState {
			p.xc.Out.Info("http.probe.ports",
				ovars{
					"count":   len(p.ports),
					"targets": strings.Join(p.ports, ","),
				})

			var cmdListPreview []string
			var cmdListTail string
			for idx, c := range p.opts.Cmds {
				cmdListPreview = append(cmdListPreview, fmt.Sprintf("%s %s", c.Method, c.Resource))
				if idx == 2 {
					cmdListTail = ",..."
					break
				}
			}

			cmdInfo := fmt.Sprintf("%s%s", strings.Join(cmdListPreview, ","), cmdListTail)
			p.xc.Out.Info("http.probe.commands",
				ovars{
					"count":    len(p.opts.Cmds),
					"commands": cmdInfo,
				})
		}

		var callFailureCount int

	probeLoop:
		for _, port := range p.ports {
			// a hacky way to check the results of the previous loop iteration
			if p.OkCount > 0 && !p.opts.Full {
				break probeLoop
			}

			var called bool
			var okCall bool
			var errCount int

			dstPort, found := p.availableHostPorts[port]
			if (found && dstPort == defaultRedisPortStr) || port == defaultRedisPortStr {
				//NOTE: a hacky way to support the Redis protocol
				//TODO: refactor and have a flag to disable this port-based behavior
				maxRetryCount := p.retryCount()
				for i := 0; i < maxRetryCount; i++ {
					output, err := redisPing(p.targetHost, port)
					p.CallCount++
					called = true

					statusCode := "error"
					callErrorStr := "none"
					if err == nil {
						statusCode = "ok"
					} else {
						callErrorStr = err.Error()
					}

					if p.printState {
						p.xc.Out.Info("redis.probe.call",
							ovars{
								"status":  statusCode,
								"output":  output,
								"port":    port,
								"attempt": i + 1,
								"error":   callErrorStr,
								"time":    time.Now().UTC().Format(time.RFC3339),
							})
					}

					if err == nil {
						okCall = true
						break
					} else {
						errCount++
					}

					time.Sleep(1 * time.Second)
				} // end of redis call retry loop
			} else if (found && dstPort == defaultDNSPortStr) || port == defaultDNSPortStr {
				//NOTE: a hacky way to support the DNS protocol
				//TODO: refactor and have a flag to disable this port-based behavior
				maxRetryCount := p.retryCount()
				for i := 0; i < maxRetryCount; i++ {
					//NOTE: use 'tcp', but later add support for 'udp' when probes support UDP
					output, err := dnsPing(context.Background(), p.targetHost, port, true)
					p.CallCount++
					called = true

					statusCode := "error"
					callErrorStr := "none"
					if err == nil {
						statusCode = "ok"
					} else {
						callErrorStr = err.Error()
					}

					if p.printState {
						p.xc.Out.Info("dns.probe.call",
							ovars{
								"status":  statusCode,
								"output":  output,
								"port":    port,
								"attempt": i + 1,
								"error":   callErrorStr,
								"time":    time.Now().UTC().Format(time.RFC3339),
							})
					}

					if err == nil {
						okCall = true
						break
					} else {
						errCount++
					}

					time.Sleep(1 * time.Second)
				} // end of DNS call retry loop
			}

			if called {
				if okCall {
					p.OkCount++
				} else {
					p.ErrCount += uint64(errCount)
					callFailureCount++
				}

				if p.opts.ExitOnFailureCount > 0 && callFailureCount >= p.opts.ExitOnFailureCount {
					if p.printState {
						p.xc.Out.Info("probe.call.failure.exit",
							ovars{
								"port":                  port,
								"port.dst":              dstPort,
								"exit.on.failure.count": p.opts.ExitOnFailureCount,
							})
					}

					break probeLoop
				}

				//If it's ok stop after the first successful probe pass
				if okCall && !p.opts.Full {
					break probeLoop
				}

				//continue to the next port to probe...
				continue
			}

			for _, cmd := range p.opts.Cmds {
				var reqBody io.Reader
				var rbSeeker io.Seeker
				var contentTypeHdr string

				if cmd.BodyFile != "" {
					_, err := os.Stat(cmd.BodyFile)
					if err != nil {
						log.Errorf("http.probe - cmd.BodyFile (%s) check error: %v", cmd.BodyFile, err)
						continue
					} else {
						bodyFile, err := os.Open(cmd.BodyFile)
						if err != nil {
							log.Errorf("http.probe - cmd.BodyFile (%s) read error: %v", cmd.BodyFile, err)
							continue
						} else {
							if cmd.BodyIsForm {
								if cmd.FormFileName == "" {
									cmd.FormFileName = filepath.Base(bodyFile.Name())
								}

								var bodyForm *bytes.Buffer
								contentTypeHdr, bodyForm, err = newFormData(cmd.FormFieldName, cmd.FormFileName, bodyFile)
								if err != nil {
									log.Errorf("http.probe - cmd.BodyFile (%s) newFormData error: %v", cmd.BodyFile, err)
									continue
								}

								br := bytes.NewReader(bodyForm.Bytes())
								reqBody = br
								rbSeeker = br
							} else {
								reqBody = bodyFile
								rbSeeker = bodyFile
							}

							//the file will be closed only when the function exits
							defer bodyFile.Close()
						}
					}
				} else {
					if cmd.BodyGenerate != "" {
						if !data.IsGenerateType(cmd.BodyGenerate) {
							cmd.BodyGenerate = data.TypeGenerateText
						}

						if cmd.BodyGenerate == data.TypeGenerateText {
							cmd.Body = data.DefaultText
						} else if cmd.BodyGenerate == data.TypeGenerateTextJSON {
							cmd.Body = data.DefaultTextJSON
						} else {
							gd, err := data.GenerateImage(cmd.BodyGenerate)
							if err != nil {
								bb := bytes.NewReader(gd)
								if cmd.BodyIsForm {
									var bodyForm *bytes.Buffer
									contentTypeHdr, bodyForm, err = newFormData(cmd.FormFieldName, cmd.FormFileName, bb)
									if err != nil {
										log.Errorf("http.probe - cmd.BodyGenerate (%s) newFormData error: %v", cmd.BodyGenerate, err)
										continue
									}

									bb = bytes.NewReader(bodyForm.Bytes())
								}

								reqBody = bb
								rbSeeker = bb
							} else {
								log.Errorf("http.probe - cmd.BodyGenerate (%s) error: %v", cmd.BodyGenerate, err)
								continue
							}
						}
					}

					if cmd.Body != "" {
						strBody := strings.NewReader(cmd.Body)
						if cmd.BodyIsForm {
							var bodyForm *bytes.Buffer
							var err error
							contentTypeHdr, bodyForm, err = newFormData(cmd.FormFieldName, cmd.FormFileName, strBody)
							if err != nil {
								log.Errorf("http.probe - cmd.Body newFormData error: %v", err)
								continue
							}

							bb := bytes.NewReader(bodyForm.Bytes())
							reqBody = bb
							rbSeeker = bb
						} else {
							reqBody = strBody
							rbSeeker = strBody
						}
					}
				}

				// TODO: need a smarter and more dynamic way to determine the actual protocol type

				// Set up FastCGI defaults if the default CGI port is used without a FastCGI config.
				if port == defaultFastCGIPortStr && cmd.FastCGI == nil {
					log.Debugf("HTTP probe - FastCGI default port (%s) used, setting up HTTP probe FastCGI wrapper defaults", port)

					// Typicall the entrypoint into a PHP app.
					if cmd.Resource == "/" {
						cmd.Resource = "/index.php"
					}

					// SplitPath is typically on the first .php path element.
					var splitPath []string
					if phpIdx := strings.Index(cmd.Resource, ".php"); phpIdx != -1 {
						splitPath = []string{cmd.Resource[:phpIdx+4]}
					}

					cmd.FastCGI = &config.FastCGIProbeWrapperConfig{
						// /var/www is a typical root for PHP indices.
						Root:      "/var/www",
						SplitPath: splitPath,
					}
				}

				var protocols []string
				if cmd.Protocol == "" {
					switch port {
					case defaultHTTPPortStr:
						protocols = []string{config.ProtoHTTP}
					case defaultHTTPSPortStr:
						protocols = []string{config.ProtoHTTPS}
					default:
						protocols = []string{config.ProtoHTTP, config.ProtoHTTPS}
					}
				} else {
					protocols = []string{cmd.Protocol}
				}

				for _, proto := range protocols {
					maxRetryCount := p.retryCount()
					notReadyErrorWait := time.Duration(16)
					webErrorWait := time.Duration(8)
					otherErrorWait := time.Duration(4)
					if p.opts.RetryWait > 0 {
						webErrorWait = time.Duration(p.opts.RetryWait)
						notReadyErrorWait = time.Duration(p.opts.RetryWait * 2)
						otherErrorWait = time.Duration(p.opts.RetryWait / 2)
					}

					if IsValidWSProto(proto) {
						wc, err := NewWebsocketClient(proto, p.targetHost, port)
						if err != nil {
							log.Debugf("HTTP probe - new websocket error - %v", err)
							continue
						}

						wc.ReadCh = make(chan WebsocketMessage, 10)
						okCall = false
						for i := 0; i < maxRetryCount; i++ {
							err = wc.Connect()
							if err != nil {
								log.Debugf("HTTP probe - ws target not ready yet (retry again later) [err=%v]...", err)
								time.Sleep(notReadyErrorWait * time.Second)
								continue
							}

							wc.CheckConnection()
							//TODO: prep data to write from the HTTPProbeCmd fields
							err = wc.WriteString("ws.data")
							p.CallCount++

							if p.printState {
								statusCode := "error"
								callErrorStr := "none"
								if err == nil {
									statusCode = "ok"
								} else {
									callErrorStr = err.Error()
								}

								p.xc.Out.Info("http.probe.call.ws",
									ovars{
										"status":    statusCode,
										"stats.rc":  wc.ReadCount,
										"stats.pic": wc.PingCount,
										"stats.poc": wc.PongCount,
										"endpoint":  wc.Addr,
										"attempt":   i + 1,
										"error":     callErrorStr,
										"time":      time.Now().UTC().Format(time.RFC3339),
									})
							}

							if err != nil {
								errCount++
								p.ErrCount++
								log.Debugf("HTTP probe - websocket write error - %v", err)
								time.Sleep(notReadyErrorWait * time.Second)
							} else {
								okCall = true
								p.OkCount++

								//try to read something from the socket
								select {
								case wsMsg := <-wc.ReadCh:
									log.Debugf("HTTP probe - websocket read - [type=%v data=%s]", wsMsg.Type, string(wsMsg.Data))
								case <-time.After(time.Second * 5):
									log.Debugf("HTTP probe - websocket read time out")
								}

								break
							}
						}

						wc.Disconnect()

						if !okCall {
							callFailureCount++
						}

						if p.opts.ExitOnFailureCount > 0 && callFailureCount >= p.opts.ExitOnFailureCount {
							if p.printState {
								p.xc.Out.Info("probe.call.failure.exit",
									ovars{
										"port":                  port,
										"port.dst":              dstPort,
										"proto":                 proto,
										"cmd":                   fmt.Sprintf("%s|%s|%s", cmd.Protocol, cmd.Method, cmd.Resource),
										"exit.on.failure.count": p.opts.ExitOnFailureCount,
									})
							}

							break probeLoop
						}

						continue
					}

					var client *http.Client
					switch {
					case cmd.FastCGI != nil:
						log.Debug("HTTP probe - FastCGI embedded proxy configured")
						client = getFastCGIClient(p.opts.ClientTimeout, cmd.FastCGI)
					default:
						var err error
						if client, err = getHTTPClient(proto, p.opts.ClientTimeout); err != nil {
							p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
							continue
						}
					}

					baseAddr := getHTTPAddr(proto, p.targetHost, port)
					// TODO: cmd.Resource may need to be a part of cmd.FastCGI instead.
					addr := fmt.Sprintf("%s%s", baseAddr, cmd.Resource)

					req, err := newHTTPRequestFromCmd(cmd, addr, reqBody)
					if err != nil {
						p.xc.Out.Error("HTTP probe - construct request error - %v", err.Error())
						continue
					}

					if contentTypeHdr != "" {
						req.Header.Set(HeaderContentType, contentTypeHdr)
					}

					okCall = false
					for i := 0; i < maxRetryCount; i++ {
						res, err := client.Do(req.Clone(context.Background()))
						p.CallCount++
						if rbSeeker != nil {
							rbSeeker.Seek(0, 0)
						}

						if res != nil {
							if res.Body != nil {
								io.Copy(io.Discard, res.Body)
							}

							res.Body.Close()
						}

						statusCode := "error"
						callErrorStr := "none"
						if err == nil {
							statusCode = fmt.Sprintf("%v", res.StatusCode)
							if p.opts.FailOnStatus5xx &&
								res.StatusCode >= 500 &&
								res.StatusCode < 600 {
								err = ErrFailOnStatus5xx
								if p.printState {
									p.xc.Out.Info("http.probe.call.status.error",
										ovars{
											"status":   statusCode,
											"method":   cmd.Method,
											"endpoint": addr,
										})
								}
							}
						} else {
							callErrorStr = err.Error()
						}

						if p.printState {
							p.xc.Out.Info("http.probe.call",
								ovars{
									"status":   statusCode,
									"method":   cmd.Method,
									"endpoint": addr,
									"attempt":  i + 1,
									"error":    callErrorStr,
									"time":     time.Now().UTC().Format(time.RFC3339),
								})
						}

						if err == nil {
							p.OkCount++
							okCall = true

							if p.OkCount == 1 {
								// running API spec probes after the first successful call
								// todo: refactor / move it after the cmd probes loop

								if len(p.opts.APISpecs) != 0 && len(p.opts.APISpecFiles) != 0 && cmd.FastCGI != nil {
									p.xc.Out.Info("HTTP probe - API spec probing not implemented for fastcgi")
								} else {
									p.probeAPISpecs(proto, p.targetHost, port)
								}
							}

							if cmd.Crawl {
								// running crawl for each probe command where it's enabled

								if cmd.FastCGI != nil {
									p.xc.Out.Info("HTTP probe - crawling not implemented for fastcgi")
								} else {
									p.crawl(proto, p.targetHost, addr)
								}
							}
							break
						} else {
							errCount++
							p.ErrCount++

							urlErr := &url.Error{}
							if errors.As(err, &urlErr) {
								if errors.Is(urlErr.Err, io.EOF) {
									log.Debugf("HTTP probe - target not ready yet (retry again later)...")
									time.Sleep(notReadyErrorWait * time.Second)
								} else {
									log.Debugf("HTTP probe - web error... retry again later...")
									time.Sleep(webErrorWait * time.Second)
								}

							} else {
								log.Debugf("HTTP probe - other error... retry again later...")
								time.Sleep(otherErrorWait * time.Second)
							}
						}
					} // call loop

					if !okCall {
						callFailureCount++
					}

					if p.opts.ExitOnFailureCount > 0 && callFailureCount >= p.opts.ExitOnFailureCount {
						if p.printState {
							p.xc.Out.Info("probe.call.failure.exit",
								ovars{
									"port":                  port,
									"port.dst":              dstPort,
									"proto":                 proto,
									"endpoint":              addr,
									"cmd":                   fmt.Sprintf("%s|%s|%s", cmd.Protocol, cmd.Method, cmd.Resource),
									"exit.on.failure.count": p.opts.ExitOnFailureCount,
								})
						}

						break probeLoop
					}
				}
			}
		}

		log.Info("HTTP probe done.")

		if p.printState {
			p.xc.Out.Info("http.probe.summary",
				ovars{
					"total":      p.CallCount,
					"failures":   p.ErrCount,
					"successful": p.OkCount,
				})

			outVars := ovars{}
			//warning := ""
			switch {
			case p.CallCount == 0:
				outVars["warning"] = "no.calls"
				//warning = "warning=no.calls"
			case p.OkCount == 0:
				//warning = "warning=no.successful.calls"
				outVars["warning"] = "no.successful.calls"
			}

			p.xc.Out.State("http.probe.done", outVars)
		}

		p.workers.Wait()
		close(p.doneChan)
	}()
}

func (p *CustomProbe) probeAPISpecs(proto, targetHost, port string) {
	//fetch the API spec when we know the target is reachable
	p.loadAPISpecs(proto, targetHost, port)

	//ideally api spec probes should work without http probe commands
	//for now trigger the api spec probes after the first successful http probe command
	//and once the api specs are loaded
	for _, specInfo := range p.APISpecProbes {
		var apiPrefix string
		if specInfo.prefixOverride != "" {
			apiPrefix = specInfo.prefixOverride
		} else {
			var err error
			apiPrefix, err = apiSpecPrefix(specInfo.spec)
			if err != nil {
				p.xc.Out.Error("http.probe.api-spec.error.prefix", err.Error())
				continue
			}
		}

		p.probeAPISpecEndpoints(proto, targetHost, port, apiPrefix, specInfo.spec)
	}
}

// DoneChan returns the 'done' channel for the HTTP probe instance
func (p *CustomProbe) DoneChan() <-chan struct{} {
	return p.doneChan
}

func newHTTPRequestFromCmd(cmd config.HTTPProbeCmd, addr string, reqBody io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(context.Background(), cmd.Method, addr, reqBody)
	if err != nil {
		return nil, err
	}

	for _, hline := range cmd.Headers {
		hparts := strings.SplitN(hline, ":", 2)
		if len(hparts) != 2 {
			log.Debugf("ignoring malformed header (%v)", hline)
			continue
		}

		hname := strings.TrimSpace(hparts[0])
		hvalue := strings.TrimSpace(hparts[1])
		req.Header.Add(hname, hvalue)
	}

	if (cmd.Username != "") || (cmd.Password != "") {
		req.SetBasicAuth(cmd.Username, cmd.Password)
	}

	return req, nil
}

func newFormData(fieldName string, fileName string, inputReader io.Reader) (string, *bytes.Buffer, error) {
	if fieldName == "" {
		fieldName = defaultFormFieldName
	}

	if fileName == "" {
		fileName = defaultFormFileName
	}

	var out bytes.Buffer
	mpw := multipart.NewWriter(&out)

	part, err := mpw.CreateFormFile(fieldName, fileName)
	if err != nil {
		return "", nil, err
	}

	if _, err = io.Copy(part, inputReader); err != nil {
		return "", nil, err
	}

	// "finalize" the form data.
	if err = mpw.Close(); err != nil {
		return "", nil, err
	}

	return mpw.FormDataContentType(), &out, nil
}
