package ops

import (
	"context"
	"time"
)

func mustInitializeRuntimeWithDeps(deps ModuleDeps) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := deps.DictionaryService.LoadCache(ctx); err != nil {
		panic("初始化字典缓存失败: " + err.Error())
	}
}
