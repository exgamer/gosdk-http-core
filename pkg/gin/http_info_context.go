package gin

import (
	"context"

	"github.com/exgamer/gosdk-http-core/pkg/config"
)

// httpInfoKey Ключ HttpInfo в контексте: свой тип не пересекается со строковыми ключами других пакетов.
type httpInfoKey struct{}

// WithHttpInfo Кладёт HttpInfo в контекст под ключом пакета; читать — GetHttpInfoFromContext.
func WithHttpInfo(ctx context.Context, info *config.HttpInfo) context.Context {
	return context.WithValue(ctx, httpInfoKey{}, info)
}

// GetHttpInfoFromContext Читает HttpInfo, положенный WithHttpInfo; nil — если ctx == nil,
// значения нет или оно не *config.HttpInfo. Строковый ключ "http_info" не читается.
func GetHttpInfoFromContext(ctx context.Context) *config.HttpInfo {
	if ctx == nil {
		return nil
	}

	if info, ok := ctx.Value(httpInfoKey{}).(*config.HttpInfo); ok {
		return info
	}

	return nil
}
