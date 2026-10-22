package gin

import (
	"context"
	"testing"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

func TestHttpInfoContext_WithHttpInfoReturnsTheSamePointer(t *testing.T) {
	info := &config.HttpInfo{}

	if got := GetHttpInfoFromContext(WithHttpInfo(context.Background(), info)); got != info {
		t.Errorf("GetHttpInfoFromContext = %p, want %p", got, info)
	}
}

// A value put under the string key "http_info" must not pass for HttpInfo.
func TestHttpInfoContext_StringKeyIsIgnored(t *testing.T) {
	spoof := &config.HttpInfo{RequestId: "spoof"}
	ctx := context.WithValue(context.Background(), "http_info", spoof) //nolint:staticcheck // SA1029: проверяем, что строковый ключ не подменяет HttpInfo

	if got := GetHttpInfoFromContext(ctx); got != nil {
		t.Errorf("GetHttpInfoFromContext = %+v, want nil for the string key", got)
	}
}

func TestHttpInfoContext_StringKeyDoesNotShadowTheOwnKey(t *testing.T) {
	own := &config.HttpInfo{RequestId: "own"}
	ctx := WithHttpInfo(context.Background(), own)
	ctx = context.WithValue(ctx, "http_info", &config.HttpInfo{RequestId: "spoof"}) //nolint:staticcheck // SA1029: проверяем, что строковый ключ не подменяет HttpInfo

	if got := GetHttpInfoFromContext(ctx); got != own {
		t.Errorf("GetHttpInfoFromContext = %+v, want the value put by WithHttpInfo", got)
	}
}

func TestHttpInfoContext_Empty(t *testing.T) {
	var nilCtx context.Context

	cases := []struct {
		name string
		ctx  context.Context
	}{
		{name: "nothing put", ctx: context.Background()},
		{name: "nil HttpInfo put", ctx: WithHttpInfo(context.Background(), nil)},
		{name: "nil context", ctx: nilCtx},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetHttpInfoFromContext(tc.ctx); got != nil {
				t.Errorf("GetHttpInfoFromContext = %+v, want nil", got)
			}
		})
	}
}
