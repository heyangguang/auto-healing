package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/company/auto-healing/internal/modules/ops/model"
	opsrepo "github.com/company/auto-healing/internal/modules/ops/repository"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DictionaryService 字典值服务（含内存缓存）
type DictionaryService struct {
	repo         *opsrepo.DictionaryRepository
	mu           sync.RWMutex
	cache        map[string][]model.Dictionary // dict_type -> []Dictionary
	cacheLoadErr error
}

type DictionaryServiceDeps struct {
	Repo *opsrepo.DictionaryRepository
}

func NewDictionaryServiceWithDB(db *gorm.DB) *DictionaryService {
	return NewDictionaryServiceWithDeps(DictionaryServiceDeps{
		Repo: opsrepo.NewDictionaryRepositoryWithDB(db),
	})
}

func NewDictionaryServiceWithDeps(deps DictionaryServiceDeps) *DictionaryService {
	if deps.Repo == nil {
		panic("dictionary service requires repository")
	}
	return &DictionaryService{
		repo: deps.Repo,
	}
}

// LoadCache 启动时加载全量缓存
func (s *DictionaryService) LoadCache(ctx context.Context) error {
	items, err := s.repo.ListByTypes(ctx, nil, true)
	if err != nil {
		return s.setDictionaryCacheError(fmt.Errorf("加载字典缓存失败: %w", err))
	}

	cache := make(map[string][]model.Dictionary)
	for _, item := range items {
		cache[item.DictType] = append(cache[item.DictType], item)
	}

	s.mu.Lock()
	s.cache = cache
	s.cacheLoadErr = nil
	s.mu.Unlock()

	logger.Info("字典缓存已加载: %d 个类型, %d 条记录", len(cache), len(items))
	return nil
}

// InvalidateCache 清除缓存
func (s *DictionaryService) InvalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.cacheLoadErr = nil
	s.mu.Unlock()
}

// GetAll 获取字典数据（优先从缓存读取）
func (s *DictionaryService) GetAll(ctx context.Context, types []string, activeOnly bool) (map[string][]model.Dictionary, error) {
	// 如果请求全量且仅活跃数据，尝试使用缓存
	if activeOnly {
		s.mu.RLock()
		cache := s.cache
		cacheErr := s.cacheLoadErr
		s.mu.RUnlock()

		if cacheErr != nil {
			return nil, cacheErr
		}
		if cache != nil {
			if len(types) == 0 {
				// 返回全量缓存
				return cloneDictionaryCache(cache), nil
			}
			// 从缓存中筛选指定类型
			result := make(map[string][]model.Dictionary)
			for _, t := range types {
				if items, ok := cache[t]; ok {
					result[t] = cloneDictionaryItems(items)
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
func (s *DictionaryService) GetTypes(ctx context.Context) ([]opsrepo.DictTypeInfo, error) {
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
	return s.LoadCache(ctx)
}

func (s *DictionaryService) setDictionaryCacheError(err error) error {
	s.mu.Lock()
	s.cache = nil
	s.cacheLoadErr = err
	s.mu.Unlock()
	logger.Error("加载字典缓存失败: %v", err)
	return err
}

func cloneDictionaryCache(cache map[string][]model.Dictionary) map[string][]model.Dictionary {
	result := make(map[string][]model.Dictionary, len(cache))
	for key, items := range cache {
		result[key] = cloneDictionaryItems(items)
	}
	return result
}

func cloneDictionaryItems(items []model.Dictionary) []model.Dictionary {
	result := make([]model.Dictionary, len(items))
	for i, item := range items {
		result[i] = item
		result[i].Extra = cloneDictionaryJSON(item.Extra)
	}
	return result
}

func cloneDictionaryJSON(src model.JSON) model.JSON {
	if src == nil {
		return nil
	}
	data, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var cloned model.JSON
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	return cloned
}
