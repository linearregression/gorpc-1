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
	OnStart func(ctx context.Context)
	OnFinish func(ctx context.Context)
	OnError func(ctx context.Context, err error)
	OnPanic func(ctx context.Context, r interface{}, trace []byte)
	OnPrepareRequest func(ctx context.Context, req *http.Request)
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

>>>CLIENT_API_FUNCS<<<

>>>CLIENT_STRUCTS<<<

type httpSessionResponse struct {
	Result string      ` + "`" + `json:"result"` + "`" + ` //OK or ERROR
	Data   json.RawMessage ` + "`" + `json:"data"` + "`" + `
	Error  int      ` + "`" + `json:"error"` + "`" + `
}

func (api *>>>API_NAME<<<) set(ctx context.Context, path string, data interface{}, buf interface{}) (err error) {
	api.callbacks.OnStart(ctx)
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			n := runtime.Stack(buf, false)
			trace := buf[:n]

			err = fmt.Errorf("panic while calling %q service: %v", api.serviceName, r)
			api.callbacks.OnPanic(ctx, r, trace)
		}
	}()

	var apiURL string
	apiURL, err = api.balancer.Next()
	if err != nil {
		api.callbacks.OnError(ctx, err)
		return err
	}

	b := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(b)
	if err := encoder.Encode(data); err != nil {
		err = fmt.Errorf("could not marshal data %+v: %v", data, err)
		api.callbacks.OnError(ctx, err)
		return err
	}

	var req *http.Request
	req, err = http.NewRequest("POST", createRawURL(apiURL, path, nil), b)
	if err != nil {
		api.callbacks.OnError(ctx, err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	api.callbacks.OnPrepareRequest(ctx, req)

	if err := doRequest(api.client, req, buf); err != nil {
		api.callbacks.OnError(ctx, err)
		return err
	}

	api.callbacks.OnFinish(ctx)
	return nil
}

func createRawURL(url, path string, values url.Values) string {
	var buf bytes.Buffer
	buf.WriteString(strings.TrimRight(url, "/"))
	buf.WriteRune('/')
	buf.WriteString(strings.TrimLeft(path, "/"))
	if len(values) > 0 {
		buf.WriteRune('?')
		buf.WriteString(values.Encode())
	}
	return buf.String()
}

func doRequest(client *http.Client, request *http.Request, buf interface{}) error {
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
		return ServiceError{
			Code: mainResp.Error,
			Message: "TODO", // TODO: extract error message from handler info, handle not found/unknown error
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
func (err ServiceError) Error() string {
	return err.Message
}
`)

var handlerCallPostFuncTemplate = []byte(`
func (api *>>>API_NAME<<<) >>>HANDLER_NAME<<<(ctx context.Context, options >>>INPUT_TYPE<<<) (*>>>RETURNED_TYPE<<<, error) {
    var result >>>RETURNED_TYPE<<<
    err := api.set(ctx, ">>>HANDLER_PATH<<<", options, &result)
	return &result, err
}
`)
