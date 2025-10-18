package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app/master/probe/data"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

type apiSpecInfo struct {
	spec           *openapi3.T
	prefixOverride string
}

func (p *CustomProbe) loadAPISpecFiles() {
	for _, info := range p.opts.APISpecFiles {
		fileName := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			fileName = parts[0]
			prefixOverride = parts[1]
		}

		spec, err := loadAPISpecFromFile(fileName)
		if err != nil {
			p.xc.Out.Info("http.probe.apispec.error",
				ovars{
					"message": "error loading api spec file",
					"error":   err,
				})
			continue
		}

		if spec == nil {
			p.xc.Out.Info("http.probe.apispec",
				ovars{
					"message": "unsupported spec type",
				})
			continue
		}

		info := apiSpecInfo{
			spec:           spec,
			prefixOverride: prefixOverride,
		}

		p.APISpecProbes = append(p.APISpecProbes, info)
	}
}

func parseAPISpec(rdata []byte) (*openapi3.T, error) {
	if isOpenAPI(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is openapi")
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		spec, err := loader.LoadFromData(rdata)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.LoadFromData - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	if isSwagger(rdata) {
		log.Debug("http.CustomProbe.parseAPISpec - is swagger")
		spec2 := &openapi2.T{}
		if err := yaml.Unmarshal(rdata, spec2); err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.yaml.Unmarshal - error=%v", err)
			return nil, err
		}

		spec, err := openapi2conv.ToV3(spec2)
		if err != nil {
			log.Debugf("http.CustomProbe.parseAPISpec.ToV3 - error=%v", err)
			return nil, err
		}

		return spec, nil
	}

	log.Debugf("http.CustomProbe.parseAPISpec - unsupported api spec type (%d): %s", len(rdata), string(rdata))
	return nil, nil
}

func loadAPISpecFromEndpoint(client *http.Client, endpoint string) (*openapi3.T, error) {
	log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint(%s)", endpoint)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.http.NewRequest - error=%v", err)
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.httpClient.Do - error=%v", err)
		return nil, err
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if res.Body != nil {
		rdata, err := io.ReadAll(res.Body)
		if err != nil {
			log.Debugf("http.CustomProbe.loadAPISpecFromEndpoint.response.read - error=%v", err)
			return nil, err
		}

		return parseAPISpec(rdata)
	}

	log.Debug("http.CustomProbe.loadAPISpecFromEndpoint.response - no body")
	return nil, nil
}

func loadAPISpecFromFile(name string) (*openapi3.T, error) {
	rdata, err := os.ReadFile(name)
	if err != nil {
		log.Debugf("http.CustomProbe.loadAPISpecFromFile.ReadFile - error=%v", err)
		return nil, err
	}

	return parseAPISpec(rdata)
}

func isSwagger(data []byte) bool {
	if (bytes.Contains(data, []byte(`"swagger":`)) ||
		bytes.Contains(data, []byte(`swagger:`))) &&
		bytes.Contains(data, []byte(`paths`)) {
		return true
	}

	return false
}

func isOpenAPI(data []byte) bool {
	if (bytes.Contains(data, []byte(`"openapi":`)) ||
		bytes.Contains(data, []byte(`openapi:`))) &&
		bytes.Contains(data, []byte(`paths`)) {
		return true
	}

	return false
}

func apiSpecPrefix(spec *openapi3.T) (string, error) {
	//for now get the api prefix from the first server struct
	//later, support multiple prefixes if there's more than one server struct
	var prefix string
	for _, sinfo := range spec.Servers {
		xurl := sinfo.URL
		if strings.Contains(xurl, "{") {
			for k, vinfo := range sinfo.Variables {
				varStr := fmt.Sprintf("{%s}", k)
				if strings.Contains(xurl, varStr) {
					valStr := "var"
					if vinfo.Default != "" {
						valStr = fmt.Sprintf("%v", vinfo.Default)
					} else if len(vinfo.Enum) > 0 {
						valStr = fmt.Sprintf("%v", vinfo.Enum[0])
					}

					xurl = strings.ReplaceAll(xurl, varStr, valStr)
				}
			}
		}

		if strings.Contains(xurl, "{") {
			xurl = strings.ReplaceAll(xurl, "{", "")

			if strings.Contains(xurl, "}") {
				xurl = strings.ReplaceAll(xurl, "}", "")
			}
		}

		parsed, err := url.Parse(xurl)
		if err != nil {
			return "", err
		}

		if parsed.Path != "" && parsed.Path != "/" {
			prefix = parsed.Path
		}
	}

	return prefix, nil
}

func (p *CustomProbe) loadAPISpecs(proto, targetHost, port string) {

	baseAddr := getHTTPAddr(proto, targetHost, port)
	client, err := getHTTPClient(proto, p.opts.ClientTimeout)
	if err != nil {
		p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
		return
	}

	//TODO:
	//Need to support user provided target port for the spec,
	//but these need to be mapped to the actual port at runtime
	//Need to support user provided target proto for the spec
	for _, info := range p.opts.APISpecs {
		specPath := info
		prefixOverride := ""
		if strings.Contains(info, ":") {
			parts := strings.SplitN(info, ":", 2)
			specPath = parts[0]
			prefixOverride = parts[1]
		}

		addr := fmt.Sprintf("%s%s", baseAddr, specPath)
		spec, err := loadAPISpecFromEndpoint(client, addr)
		if err != nil {
			p.xc.Out.Info("http.probe.apispec.error",
				ovars{
					"message": "error loading api spec from endpoint",
					"error":   err,
				})

			continue
		}

		if spec == nil {
			p.xc.Out.Info("http.probe.apispec",
				ovars{
					"message": "unsupported spec type",
				})

			continue
		}

		info := apiSpecInfo{
			spec:           spec,
			prefixOverride: prefixOverride,
		}

		p.APISpecProbes = append(p.APISpecProbes, info)
	}
}

func pathOps(pinfo *openapi3.PathItem) map[string]*openapi3.Operation {
	ops := map[string]*openapi3.Operation{}
	addPathOp(&ops, pinfo.Connect, "connect")
	addPathOp(&ops, pinfo.Delete, "delete")
	addPathOp(&ops, pinfo.Get, "get")
	addPathOp(&ops, pinfo.Head, "head")
	addPathOp(&ops, pinfo.Options, "options")
	addPathOp(&ops, pinfo.Patch, "patch")
	addPathOp(&ops, pinfo.Post, "post")
	addPathOp(&ops, pinfo.Put, "put")
	addPathOp(&ops, pinfo.Trace, "trace")
	return ops
}

func addPathOp(m *map[string]*openapi3.Operation, op *openapi3.Operation, name string) {
	if op != nil {
		(*m)[name] = op
	}
}

// collectParameters merges path-level and operation-level parameters.
// Operation-level parameters override path-level ones with the same (in,name).
func collectParameters(pathItem *openapi3.PathItem, op *openapi3.Operation) []*openapi3.Parameter {
	var result []*openapi3.Parameter
	// index to handle overrides
	key := func(p *openapi3.Parameter) string { return p.In + "\x00" + p.Name }
	seen := map[string]bool{}

	if pathItem != nil {
		for _, pref := range pathItem.Parameters {
			if pref == nil || pref.Value == nil {
				continue
			}
			p := pref.Value
			result = append(result, p)
			seen[key(p)] = true
		}
	}

	if op != nil {
		for _, pref := range op.Parameters {
			if pref == nil || pref.Value == nil {
				continue
			}
			p := pref.Value
			k := key(p)
			if seen[k] {
				for i := range result {
					// override by replacing prior entry
					if key(result[i]) == k {
						result[i] = p
						// OpenAPI params are unique per operation by (in,name). An op-level param
						// overrides at most one path-level entry, so replace once and stop.
						break
					}
				}
			} else {
				result = append(result, p)
				seen[k] = true
			}
		}
	}

	return result
}

func paramStringForSchema(sref *openapi3.SchemaRef) string {
	if sref == nil || sref.Value == nil {
		return "x"
	}
	s := sref.Value

	if len(s.Enum) > 0 {
		if v, ok := s.Enum[0].(string); ok {
			return v
		}
		return "1"
	}

	switch s.Type {
	case "integer", "number":
		return "1"
	case "boolean":
		return "true"
	case "array":
		return paramStringForSchema(s.Items)
	case "object":
		return "x"
	default:
		return "x"
	}
}

func substitutePathParams(apiPath string, params []*openapi3.Parameter) string {
	if !strings.Contains(apiPath, "{") {
		return apiPath
	}

	for _, p := range params {
		if p == nil || p.In != "path" {
			continue
		}
		placeholder := "{" + p.Name + "}"
		if strings.Contains(apiPath, placeholder) {
			apiPath = strings.ReplaceAll(apiPath, placeholder, url.PathEscape(paramStringForSchema(p.Schema)))
		}
	}

	// fallback: strip any remaining braces
	if strings.Contains(apiPath, "{") {
		apiPath = strings.ReplaceAll(apiPath, "{", "")
		apiPath = strings.ReplaceAll(apiPath, "}", "")
	}

	return apiPath
}

func buildQueryAndHeaders(params []*openapi3.Parameter) (string, map[string]string) {
	var parts []string
	headers := make(map[string]string)
	for _, p := range params {
		if p == nil {
			continue
		}

		getStringValue := func(pref *openapi3.SchemaRef) string {
			if pref != nil && pref.Value != nil && pref.Value.Type == "object" {
				// generate a small object and JSON-stringify it
				obj, _ := genSchemaObject(pref.Value, false)
				if data, err := json.Marshal(obj); err == nil {
					return string(data)
				}
				return "{}"
			}
			return paramStringForSchema(pref)
		}

		switch p.In {
		case "query":
			v := getStringValue(p.Schema)
			parts = append(parts, url.QueryEscape(p.Name)+"="+url.QueryEscape(v))
		case "header":
			v := getStringValue(p.Schema)
			headers[p.Name] = v
		}
	}
	return strings.Join(parts, "&"), headers
}

func genSchemaObject(schema *openapi3.Schema, minimal bool) (interface{}, bool) {
	//todo: also need 'max' as a param to generate as many fields as possible

	if schema.Type != "" {
		// todo: also use
		// schema.Format, schema.Enum, schema.Default, schema.Example, schema.Required, etc
		switch schema.Type {
		case "object":
			if schema.Example != nil {
				log.WithFields(log.Fields{
					"op":      "genSchemaObject",
					"title":   schema.Title,
					"example": fmt.Sprintf("%#v", schema.Example),
					"default": fmt.Sprintf("%#v", schema.Default),
				}).Debug("schema.Type.object")
			}

			if schema.Example != nil {
				return schema.Example, false
			}

			if schema.Default != nil {
				return schema.Default, false
			}

			obj := make(map[string]interface{})
			requiredProps := map[string]struct{}{}
			for _, pname := range schema.Required {
				requiredProps[pname] = struct{}{}
			}

			for name, prop := range schema.Properties {
				if minimal {
					_, found := requiredProps[name]
					if !found {
						continue
					}
				}

				objVal, skip := genSchemaObject(prop.Value, minimal)
				if !skip {
					obj[name] = objVal
				}
			}
			return obj, false
		case "array":
			if schema.Items != nil {
				arr, skip := genSchemaObject(schema.Items.Value, minimal)
				return []interface{}{arr}, skip
			}
		case "string":
			stringVal := "string"
			if schema.Example != nil {
				stringVal, _ = schema.Example.(string)
			} else if schema.Default != nil {
				stringVal, _ = schema.Default.(string)
			} else if len(schema.Enum) > 0 {
				stringVal, _ = schema.Enum[0].(string)
			}

			return stringVal, false
		case "integer":
			return 0, false
		case "number":
			return 0.0, false
		case "boolean":
			return true, false
		}
	}

	if len(schema.AnyOf) > 0 {
		hasNullType := false
		var v *openapi3.Schema
		for _, current := range schema.AnyOf {
			if current.Value == nil {
				continue
			}

			if current.Value.Type == "null" {
				// anyOf[].type=="null" is used for optional values
				hasNullType = true
			} else {
				if v == nil {
					v = current.Value
				}
			}
		}

		if hasNullType && minimal {
			return nil, true
		}

		return genSchemaObject(v, minimal)
	}

	//todo: handle schema.OneOf, schema.AllOf
	return nil, false
}

func (p *CustomProbe) probeAPISpecEndpoints(proto, targetHost, port, prefix string, spec *openapi3.T) {
	const op = "probe.http.CustomProbe.probeAPISpecEndpoints"
	addr := getHTTPAddr(proto, targetHost, port)

	if p.printState {
		p.xc.Out.State("http.probe.api-spec.probe.endpoint.starting",
			ovars{
				"addr":      addr,
				"prefix":    prefix,
				"endpoints": len(spec.Paths),
			})
	}

	httpClient, err := getHTTPClient(proto, p.opts.ClientTimeout)
	if err != nil {
		p.xc.Out.Error("HTTP probe - construct client error - %v", err.Error())
		return
	}

	for apiPath, pathInfo := range spec.Paths {
		rawRoute := apiPath
		// Path param substitution is handled per operation below.

		ops := pathOps(pathInfo)
		for apiMethod, apiInfo := range ops {
			if apiInfo == nil {
				log.WithFields(log.Fields{
					"op": op,
				}).Debug("no.apiInfo")
				continue
			}

			// Build endpoint for this operation using dummy path/query params
			params := collectParameters(pathInfo, apiInfo)
			finalPath := substitutePathParams(rawRoute, params)
			endpoint := fmt.Sprintf("%s%s%s", addr, prefix, finalPath)
			qstr, hdrs := buildQueryAndHeaders(params)
			if qstr != "" {
				if strings.Contains(endpoint, "?") {
					endpoint = endpoint + "&" + qstr
				} else {
					endpoint = endpoint + "?" + qstr
				}
			}

			var bodyBytes []byte
			var contentType string
			var formFieldName string
			var reqObjSchema *openapi3.Schema

			//apiInfo.Parameters
			if apiInfo.RequestBody != nil {
				if apiInfo.RequestBody.Ref != "" {
					log.WithFields(log.Fields{
						"op":   op,
						"data": apiInfo.RequestBody.Ref,
					}).Debug("apiInfo.RequestBody.Ref")
				}

				if apiInfo.RequestBody.Value != nil {
					if apiInfo.RequestBody.Value.Required {
						for mediaTypeKey, mediaTypeVal := range apiInfo.RequestBody.Value.Content {
							log.WithFields(log.Fields{
								"op":             op,
								"media.type.key": mediaTypeKey,
							}).Debug("apiInfo.RequestBody.Value.Content")

							if mediaTypeVal.Schema.Ref != "" {
								log.WithFields(log.Fields{
									"op":   op,
									"data": mediaTypeVal.Schema.Ref,
								}).Debug("mediaTypeVal.Schema.Ref")
								//the ref should be already resolved
							}

							if mediaTypeKey == "application/json" {
								if mediaTypeVal.Schema.Value != nil {
									log.WithFields(log.Fields{
										"op":   op,
										"data": mediaTypeVal.Schema.Value,
									}).Debug("mediaTypeVal.Schema.Value")
								}

								reqObj, _ := genSchemaObject(mediaTypeVal.Schema.Value, false)
								contentType = mediaTypeKey
								bodyBytes = jsonutil.ToBytes(reqObj)
								log.WithError(err).WithFields(log.Fields{
									"op":           op,
									"content.type": contentType,
									"data":         string(bodyBytes),
								}).Debug("generatedSchemaObject")

								reqObjSchema = mediaTypeVal.Schema.Value
							}

							if mediaTypeKey == "multipart/form-data" {
								formFieldName = defaultFormFieldName

								if mediaTypeVal.Schema.Value != nil {
									log.WithFields(log.Fields{
										"op":   op,
										"data": mediaTypeVal.Schema.Value,
									}).Debug("mediaTypeVal.Schema.Value")

									for pname, pval := range mediaTypeVal.Schema.Value.Properties {
										log.WithFields(log.Fields{
											"pname": pname,
											"pval":  pval,
										}).Debug("mediaTypeVal.Schema.Value.Properties")

										formFieldName = pname
										//keep iterating to see all params
										//todo: need to handle multiple params
									}
								}
							}
						}
						//
					}
				}
			}

			log.WithFields(log.Fields{
				"op":                  op,
				"api.method":          apiMethod,
				"api.endpoint":        endpoint,
				"api.op":              apiInfo.OperationID,
				"api.form.field_name": formFieldName,
			}).Debug("call p.apiSpecEndpointCall")

			if formFieldName != "" {
				strBody := strings.NewReader(data.DefaultText)
				var bodyForm *bytes.Buffer
				contentType, bodyForm, err = newFormData(formFieldName, defaultFormFileName, strBody)
				if err != nil {
					log.WithError(err).WithFields(log.Fields{
						"op": op,
					}).Error("newFormData")
				} else {
					bodyBytes = bodyForm.Bytes()
				}
			}

			// include Content-Type in headers map as needed
			if contentType != "" {
				if hdrs == nil {
					hdrs = map[string]string{}
				}
				hdrs[HeaderContentType] = contentType
			}

			//make a call
			if p.apiSpecEndpointCall(httpClient, endpoint, apiMethod, contentType, bodyBytes, hdrs) {
				if formFieldName != "" {
					//trying again with a different generated body (simple hacky version)
					//retrying only for form data for now
					gd, err := data.GenerateImage(data.TypeGenerateImage)
					if err != nil {
						log.WithError(err).WithFields(log.Fields{
							"op": op,
						}).Error("data.GenerateImage")
					} else {
						var bodyForm *bytes.Buffer
						contentType, bodyForm, err = newFormData(formFieldName, defaultFormFileName, bytes.NewReader(gd))
						if err != nil {
							log.WithError(err).WithFields(log.Fields{
								"op": op,
							}).Error("newFormData")
						} else {
							log.WithFields(log.Fields{
								"op": op,
							}).Debug("retrying.form.submit.image p.apiSpecEndpointCall")

							if p.apiSpecEndpointCall(httpClient, endpoint, apiMethod, contentType, bodyForm.Bytes(), hdrs) &&
								formFieldName != "" {
								strBody := strings.NewReader(data.DefaultTextJSON)
								var bodyForm *bytes.Buffer
								contentType, bodyForm, err = newFormData(formFieldName, defaultFormFileName, strBody)
								if err != nil {
									log.WithError(err).WithFields(log.Fields{
										"op": op,
									}).Error("newFormData")
								} else {
									log.WithFields(log.Fields{
										"op": op,
									}).Debug("retrying.form.submit.json p.apiSpecEndpointCall")
									p.apiSpecEndpointCall(httpClient, endpoint, apiMethod, contentType, bodyForm.Bytes(), hdrs)
								}
							}
						}
					}
				} else if reqObjSchema != nil {
					//try generating a minimal object this time
					reqObj, _ := genSchemaObject(reqObjSchema, true)

					bodyBytes = jsonutil.ToBytes(reqObj)
					log.WithError(err).WithFields(log.Fields{
						"op":           op,
						"content.type": contentType,
						"data":         string(bodyBytes),
					}).Debug("generatedSchemaObject(true)/retrying.post p.apiSpecEndpointCall")

					p.apiSpecEndpointCall(httpClient, endpoint, apiMethod, contentType, bodyBytes, hdrs)
				}
			}
		}
	}
}

func (p *CustomProbe) apiSpecEndpointCall(
	client *http.Client,
	endpoint string,
	method string,
	contentType string,
	bodyBytes []byte,
	headers map[string]string,
) bool {
	maxRetryCount := p.retryCount()

	notReadyErrorWait := time.Duration(16)
	webErrorWait := time.Duration(8)
	otherErrorWait := time.Duration(4)
	if p.opts.RetryWait > 0 {
		webErrorWait = time.Duration(p.opts.RetryWait)
		notReadyErrorWait = time.Duration(p.opts.RetryWait * 2)
		otherErrorWait = time.Duration(p.opts.RetryWait / 2)
	}

	method = strings.ToUpper(method)
	for i := 0; i < maxRetryCount; i++ {
		var reqBody io.Reader
		if len(bodyBytes) > 0 {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequest(method, endpoint, reqBody)
		if err != nil {
			p.xc.Out.Error("HTTP probe - construct request error - %v", err.Error())
			// Break since the same args are passed to NewRequest() on each loop.
			break
		}

		if contentType != "" {
			req.Header.Set(HeaderContentType, contentType)
		}

		for hname, hvalue := range headers {
			if strings.EqualFold(hname, HeaderContentType) {
				continue
			}
			req.Header.Add(hname, hvalue)
		}

		//no credentials for now
		res, err := client.Do(req)
		p.CallCount.Inc()

		if res != nil {
			if res.Body != nil {
				io.Copy(io.Discard, res.Body)
			}

			defer res.Body.Close()
		}

		statusCode := "error"
		callErrorStr := "none"
		if err == nil {
			if res != nil {
				statusCode = fmt.Sprintf("%v", res.StatusCode)
			}
		} else {
			callErrorStr = err.Error()
		}

		if p.printState {
			p.xc.Out.Info("http.probe.api-spec.probe.endpoint.call",
				ovars{
					"status":   statusCode,
					"method":   method,
					"endpoint": endpoint,
					"attempt":  i + 1,
					"error":    callErrorStr,
					"time":     time.Now().UTC().Format(time.RFC3339),
				})
		}

		if res != nil {
			if p.opts.FailOnStatus5xx &&
				res.StatusCode >= 500 &&
				res.StatusCode < 600 {
				err = ErrFailOnStatus5xx
				if p.printState {
					p.xc.Out.Info("http.probe.api-spec.probe.endpoint.call.status.error",
						ovars{
							"status":   statusCode,
							"method":   method,
							"endpoint": endpoint,
						})
				}
			} else if p.opts.FailOnStatus4xx &&
				res.StatusCode >= 400 &&
				res.StatusCode < 500 {
				err = ErrFailOnStatus4xx
				if p.printState {
					p.xc.Out.Info("http.probe.api-spec.probe.endpoint.call.status.error.4xx",
						ovars{
							"status":   statusCode,
							"method":   method,
							"endpoint": endpoint,
						})
				}
			} else if res.StatusCode == http.StatusInternalServerError {
				if p.printState {
					p.xc.Out.Info("http.probe.api-spec.probe.endpoint.call.status.500.try.again.with.different.data",
						ovars{
							"method":   method,
							"endpoint": endpoint,
						})
				}
				return true
			}
		}

		if err == nil {
			p.OkCount.Inc()
			break
		} else {
			p.ErrCount.Inc()

			if urlErr, ok := err.(*url.Error); ok {
				if urlErr.Err == io.EOF {
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

	}

	return false
}
