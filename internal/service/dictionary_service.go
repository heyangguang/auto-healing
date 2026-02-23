package service

import (
	"context"
	"sync"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// DictionaryService 字典值服务（含内存缓存）
type DictionaryService struct {
	repo  *repository.DictionaryRepository
	mu    sync.RWMutex
	cache map[string][]model.Dictionary // dict_type -> []Dictionary
}

// NewDictionaryService 创建服务
func NewDictionaryService() *DictionaryService {
	return &DictionaryService{
		repo: repository.NewDictionaryRepository(),
	}
}

// LoadCache 启动时加载全量缓存
func (s *DictionaryService) LoadCache(ctx context.Context) {
	items, err := s.repo.ListByTypes(ctx, nil, true)
	if err != nil {
		logger.Warn("加载字典缓存失败: %v", err)
		return
	}

	cache := make(map[string][]model.Dictionary)
	for _, item := range items {
		cache[item.DictType] = append(cache[item.DictType], item)
	}

	s.mu.Lock()
	s.cache = cache
	s.mu.Unlock()

	logger.Info("字典缓存已加载: %d 个类型, %d 条记录", len(cache), len(items))
}

// InvalidateCache 清除缓存
func (s *DictionaryService) InvalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// GetAll 获取字典数据（优先从缓存读取）
func (s *DictionaryService) GetAll(ctx context.Context, types []string, activeOnly bool) (map[string][]model.Dictionary, error) {
	// 如果请求全量且仅活跃数据，尝试使用缓存
	if activeOnly {
		s.mu.RLock()
		cache := s.cache
		s.mu.RUnlock()

		if cache != nil {
			if len(types) == 0 {
				// 返回全量缓存
				return cache, nil
			}
			// 从缓存中筛选指定类型
			result := make(map[string][]model.Dictionary)
			for _, t := range types {
				if items, ok := cache[t]; ok {
					result[t] = items
				}
			}
			return result, nil
		}
	}

	// 缓存不可用，从数据库查询
	items, err := s.repo.ListByTypes(ctx, types, activeOnly)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]model.Dictionary)
	for _, item := range items {
		result[item.DictType] = append(result[item.DictType], item)
	}
	return result, nil
}

// GetTypes 查询可用类型列表
func (s *DictionaryService) GetTypes(ctx context.Context) ([]repository.DictTypeInfo, error) {
	return s.repo.ListTypes(ctx)
}

// Create 创建字典项
func (s *DictionaryService) Create(ctx context.Context, item *model.Dictionary) error {
	err := s.repo.Create(ctx, item)
	if err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// Update 更新字典项
func (s *DictionaryService) Update(ctx context.Context, item *model.Dictionary) error {
	err := s.repo.Update(ctx, item)
	if err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// Delete 删除字典项
func (s *DictionaryService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// GetByID 根据ID查询
func (s *DictionaryService) GetByID(ctx context.Context, id uuid.UUID) (*model.Dictionary, error) {
	return s.repo.GetByID(ctx, id)
}

// SeedDictionaries Seed 字典数据
func (s *DictionaryService) SeedDictionaries(ctx context.Context) error {
	err := s.repo.UpsertBatch(ctx, AllDictionarySeeds)
	if err != nil {
		return err
	}
	s.InvalidateCache()
	s.LoadCache(ctx)
	return nil
}
