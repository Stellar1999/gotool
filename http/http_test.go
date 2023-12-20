package http

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func Test_resolveUrlWithParameter(t *testing.T) {
	type args struct {
		urlString  string
		parameters map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "normal.",
			args: args{
				urlString: "http://127.0.0.1:38080",
				parameters: map[string]string{
					"msg": "{\"a\":1}",
				},
			},
			want:    "http://127.0.0.1:38080?msg=%7B%22a%22%3A1%7D",
			wantErr: false,
		},
		{
			name: "url with queryParam",
			args: args{
				urlString: "http://127.0.0.1:38080?example=1",
				parameters: map[string]string{
					"msg": "{\"a\":1}",
				},
			},
			want:    "http://127.0.0.1:38080?example=1&msg=%7B%22a%22%3A1%7D",
			wantErr: false,
		},
		{
			name: "url with simple",
			args: args{
				urlString: "www.example.com",
				parameters: map[string]string{
					"msg": "{\"a\":1}",
				},
			},
			want:    "www.example.com?msg=%7B%22a%22%3A1%7D",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveUrlWithParameter(tt.args.urlString, tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveUrlWithParameter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolveUrlWithParameter() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// add time Hook
type TimeHook struct {
}

const TimeKey = "time-hook-count-key"

func (TimeHook) Before(ctx context.Context, req *http.Request) (context.Context, error) {
	ctx = context.WithValue(ctx, TimeKey, time.Now())
	return ctx, nil
}
func (TimeHook) After(ctx context.Context, respCode int, respHeader http.Header, respData any, err error) (context.Context, error) {
	lastTime := ctx.Value(TimeKey).(time.Time)
	fmt.Printf("cost time:%.2f s\n", time.Now().Sub(lastTime).Seconds())
	return ctx, nil
}

func TestGet(t *testing.T) {
	type args struct {
		url       string
		header    map[string]string
		parameter map[string]string
	}
	// all the test case power by JSON.Placeholder
	tests := []struct {
		name         string
		args         args
		wantRespCode int
		wantErr      bool
	}{
		{
			name: "Get Simple",
			args: args{
				url:       "http://jsonplaceholder.typicode.com/posts",
				header:    map[string]string{"test": "test"},
				parameter: nil,
			},
			wantRespCode: http.StatusOK,
			wantErr:      false,
		},
	}

	AddHook(TimeHook{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _, err := Get(tt.args.url, tt.args.header, tt.args.parameter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantRespCode {
				t.Errorf("Get() got = %v, wantRespCode %v", got, tt.wantRespCode)
			}
		})
	}
}
