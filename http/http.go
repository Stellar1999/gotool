package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	gourl "net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxIdleConns        int = 100
	DefaultMaxIdleConnsPerHost int = 100
	DefaultIdleConnTimeout     int = 90
)

type RequestMethodType string

const (
	POST   RequestMethodType = "POST"
	GET    RequestMethodType = "GET"
	PATCH  RequestMethodType = "PATCH"
	PUT    RequestMethodType = "PUT"
	DELETE RequestMethodType = "DELETE"
)

var httpClient = createHTTPClient()

type Hook interface {
	Before(ctx context.Context, req *http.Request) (context.Context, error)
	After(ctx context.Context, respCode int, respHeader http.Header, respData any, err error) (context.Context, error)
}

// global hook
var globalHttpHook []Hook

func AddHook(httpHook Hook) {
	globalHttpHook = append(globalHttpHook, httpHook)
}

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        DefaultMaxIdleConns,
			MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
			IdleConnTimeout:     time.Duration(DefaultIdleConnTimeout) * time.Second,
		},

		Timeout: 20 * time.Second,
	}
	return client
}

// SetHTTPClient this method use to init http client
func SetHTTPClient(client *http.Client) {
	httpClient = client
}

func Get(url string, header map[string]string, parameter map[string]string) (int, http.Header, any, error) {
	return GetWithContext(context.Background(), url, header, parameter)
}

func GetWithContext(ctx context.Context, url string, header map[string]string, parameter map[string]string) (int, http.Header, any, error) {
	// resolve url
	return send(ctx, GET, url, header, parameter, nil)
}

func Post(url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return PostWithContext(context.Background(), url, header, parameter, body)
}

func PostWithContext(ctx context.Context, url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return send(ctx, POST, url, header, parameter, body)
}

func Patch(url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return PatchWithContext(context.Background(), url, header, parameter, body)
}

func PatchWithContext(ctx context.Context, url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return send(ctx, PATCH, url, header, parameter, body)
}

func Put(url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return PutWithContext(context.Background(), url, header, parameter, body)
}

func PutWithContext(ctx context.Context, url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return send(ctx, PUT, url, header, parameter, body)
}

func Delete(url string, header map[string]string, parameter map[string]string) (int, http.Header, any, error) {
	return DeleteWithContext(context.Background(), url, header, parameter, nil)
}

func DeleteWithContext(ctx context.Context, url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	return send(ctx, DELETE, url, header, parameter, body)
}

func send(ctx context.Context, method RequestMethodType, url string, header map[string]string, parameter map[string]string, body any) (int, http.Header, any, error) {
	// resolve url
	url, err := resolveUrlWithParameter(url, parameter)
	if err != nil {
		return 0, nil, nil, err
	}
	var httpRequest *http.Request
	if method == POST || method == PUT || method == PATCH {
		bytes, _ := json.Marshal(body)
		payload := strings.NewReader(string(bytes))
		httpRequest, err = http.NewRequest(string(method), url, payload)
	} else {
		httpRequest, err = http.NewRequest(string(method), url, nil)
	}
	if err != nil {
		log.Printf("NewRequest error(%v)\n", err)
		return -1, nil, nil, err
	}

	if header != nil {
		httpRequest.Header = mapHeader2netHeader(header)
	}
	return do(ctx, httpRequest)
}

func do(ctx context.Context, httpRequest *http.Request) (int, http.Header, any, error) {
	for _, hook := range globalHttpHook {
		_ctx, err := hook.Before(ctx, httpRequest)
		ctx = _ctx
		if err != nil {
			return -1, nil, nil, err
		}
	}
	resp, err := httpClient.Do(httpRequest)
	if err != nil {
		return -1, nil, nil, err
	}
	rspCode, rspHead, rspData, err := doParseResponse(resp, err)
	for _, hook := range globalHttpHook {
		_ctx, err := hook.After(ctx, rspCode, rspHead, rspData, err)
		ctx = _ctx
		if err != nil {
			return -1, nil, nil, err
		}
	}
	return rspCode, rspHead, rspData, err
}

func resolveUrlWithParameter(urlString string, parameters map[string]string) (string, error) {
	url, err := gourl.Parse(urlString)
	if err != nil {
		return "", err
	}
	queryValue := url.Query()
	for key, value := range parameters {
		queryValue.Set(key, value)
	}
	url.RawQuery = queryValue.Encode()
	return url.String(), err
}

func doParseResponse(httpResponse *http.Response, err error) (int, http.Header, any, error) {
	if err != nil && httpResponse == nil {
		log.Printf("Error sending request to API endpoint. %+v", err)
		return -1, nil, nil, err
	} else {
		if httpResponse == nil {
			log.Printf("httpResponse is nil\n")
			return -1, nil, nil, nil
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Printf("Body close error(%v)", err)
			}
		}(httpResponse.Body)

		code := httpResponse.StatusCode
		headers := httpResponse.Header
		if code != http.StatusOK {
			body, _ := io.ReadAll(httpResponse.Body)
			return code, headers, nil, errors.New("remote error, url: code " + strconv.Itoa(code) + ", response body: " + string(body))
		}

		// We have seen inconsistencies even when we get 200 OK response
		body, err := io.ReadAll(httpResponse.Body)
		if err != nil {
			log.Printf("Couldn't parse response body(%v)", err)
			return code, headers, nil, errors.New("Couldn't parse response body, err: " + err.Error())
		}

		return code, headers, body, nil
	}
}

func mapHeader2netHeader(header map[string]string) http.Header {
	var netHeader = make(http.Header)
	if header != nil {
		for k, v := range header {
			netHeader.Set(k, v)
		}
	}
	return netHeader
}
