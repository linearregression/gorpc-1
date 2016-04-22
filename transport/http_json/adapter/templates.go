package adapter

var mainImports = []string{
	"bytes",
	"encoding/json",
	"fmt",
	"io/ioutil",
	"net/http",
	"net/url",
	"runtime",
	"strings",
	"time",
	"golang.org/x/net/context",
}

var mainTemplate = []byte(`
// It's autogenerated file. It's not recommended to modify it.
package >>>PKG_NAME<<<

import (
    >>>IMPORTS<<<
)

type IBalancer interface {
    Next() (string, error)
}

type Callbacks struct {
	OnStart          func(ctx context.Context, req *http.Request) context.Context
	OnPrepareRequest func(ctx context.Context, req *http.Request, data interface{}) context.Context
	OnSuccess        func(ctx context.Context, req *http.Request, data interface{})
	OnError          func(ctx context.Context, req *http.Request, err error)
	OnPanic          func(ctx context.Context, req *http.Request, r interface{}, trace []byte)
	OnFinish         func(ctx context.Context, req *http.Request, startTime time.Time)
}

type >>>API_NAME<<< struct {
	client      *http.Client
	serviceName string
	balancer    IBalancer
	callbacks   Callbacks
}

func New>>>API_NAME<<<(client *http.Client, balancer IBalancer, callbacks Callbacks) *>>>API_NAME<<< {
	if client == nil {
		client = http.DefaultClient
	}
	return &>>>API_NAME<<<{
//		client: &http.Client{
//			Transport: &http.Transport{
//				//DisableCompression: true,
//				MaxIdleConnsPerHost: 20,
//			},
//			Timeout: apiTimeout,
//		},
		serviceName: ">>>API_NAME<<<",
		balancer:    balancer,
		callbacks:   callbacks,
		client:      client,
	}
}

>>>CLIENT_API<<<

// TODO: duplicates http_json.httpSessionResponse
type httpSessionResponse struct {
	Result string      ` + "`" + `json:"result"` + "`" + ` //OK or ERROR
	Data   json.RawMessage ` + "`" + `json:"data"` + "`" + `
	Error  string      ` + "`" + `json:"error"` + "`" + `
}

func (api *>>>API_NAME<<<) set(ctx context.Context, path string, data interface{}, buf interface{}, handlerErrors map[string]int) (err error) {
	startTime := time.Now()

	var apiURL string
	var req *http.Request

	if api.callbacks.OnStart != nil {
		ctx = api.callbacks.OnStart(ctx, req)
	}

	defer func() {
		if api.callbacks.OnFinish != nil {
			api.callbacks.OnFinish(ctx, req, startTime)
		}

		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			n := runtime.Stack(buf, false)
			trace := buf[:n]

			err = fmt.Errorf("panic while calling %q service: %v", api.serviceName, r)
			if api.callbacks.OnPanic != nil {
				api.callbacks.OnPanic(ctx, req, r, trace)
			}
		}
	}()

	apiURL, err = api.balancer.Next()
	if err != nil {
		if api.callbacks.OnError != nil {
			api.callbacks.OnError(ctx, req, err)
		}
		return err
	}

	b := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(b)
	if err := encoder.Encode(data); err != nil {
		err = fmt.Errorf("could not marshal data %+v: %v", data, err)
		if api.callbacks.OnError != nil {
			api.callbacks.OnError(ctx, req, err)
		}
		return err
	}

	req, err = http.NewRequest("POST", createRawURL(apiURL, path, nil), b)
	if err != nil {
		if api.callbacks.OnError != nil {
			api.callbacks.OnError(ctx, req, err)
		}
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if api.callbacks.OnPrepareRequest != nil {
		ctx = api.callbacks.OnPrepareRequest(ctx, req)
	}

	if err := doRequest(api.client, req, buf, handlerErrors); err != nil {
		if api.callbacks.OnError != nil {
			api.callbacks.OnError(ctx, req, err)
		}
		return err
	}

	if api.callbacks.OnSuccess != nil {
		api.callbacks.OnSuccess(ctx, req, buf)
	}

	return nil
}

func createRawURL(url, path string, values url.Values) string {
	var buf bytes.Buffer
	buf.WriteString(strings.TrimRight(url, "/"))
	//buf.WriteRune('/')
	//buf.WriteString(strings.TrimLeft(path, "/"))
	// path must contain leading /
	buf.WriteString(path)
	if len(values) > 0 {
		buf.WriteRune('?')
		buf.WriteString(values.Encode())
	}
	return buf.String()
}

func doRequest(client *http.Client, request *http.Request, buf interface{}, handlerErrors map[string]int) error {
	// Run
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// Handle error
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		// TODO separate error types for different status codes (and different callbacks)
		/*
		   case http.StatusForbidden:
		   case http.StatusBadGateway:
		   case http.StatusBadRequest:
		*/
		default:
			return fmt.Errorf("Request %q failed. Server returns status code %d", request.URL.RequestURI(), response.StatusCode)
		}
	}

	// Read response
	result, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	var mainResp httpSessionResponse
	if err := json.Unmarshal(result, &mainResp); err != nil {
		return fmt.Errorf("request %q failed to decode response %q: %v", request.URL.RequestURI(), string(result), err)
	}

	if mainResp.Result == "OK" {
		if err := json.Unmarshal(mainResp.Data, buf); err != nil {
			return fmt.Errorf("request %q failed to decode response data %+v: %v", request.URL.RequestURI(), mainResp.Data, err)
		}
		return nil
	}

	if mainResp.Result == "ERROR" {
		errCode, ok := handlerErrors[mainResp.Error]
		if ok {
			return &ServiceError{
				Code: errCode,
				Message: mainResp.Error,
			}
		}
	}

	return fmt.Errorf("request %q returned incorrect response %q", request.URL.RequestURI(), string(result))
}

// ServiceError uses to separate critical and non-critical errors which returns in external service response.
// For this type of error we shouldn't use 500 error counter for librato
type ServiceError struct {
	Code    int
	Message string
}

// Error method for implementing common error interface
func (err *ServiceError) Error() string {
	return err.Message
}
`)

var handlerCallPostFuncTemplate = []byte(`
func (api *>>>API_NAME<<<) >>>HANDLER_NAME<<<(ctx context.Context, options >>>INPUT_TYPE<<<) (>>>RETURNED_TYPE<<<, error) {
    var result >>>RETURNED_TYPE<<<
    err := api.set(ctx, ">>>HANDLER_PATH<<<", options, &result, >>>HANDLER_ERRORS<<<)
	return result, err
}
`)
