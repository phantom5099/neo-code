package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"go-llm-demo/internal/server/domain"

	"gopkg.in/yaml.v3"
)

type SecurityConfigRepository interface {
	LoadAll(configDir string) (*domain.Config, *domain.Config, *domain.Config, error)
}

type securityConfigRepositoryImpl struct{}

func NewSecurityConfigRepository() SecurityConfigRepository {
	return &securityConfigRepositoryImpl{}
}

func (r *securityConfigRepositoryImpl) LoadAll(configDir string) (*domain.Config, *domain.Config, *domain.Config, error) {
	blackList, err := r.loadConfig(filepath.Join(configDir, "blacklist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载黑名单失败：%w", err)
	}

	whiteList, err := r.loadConfig(filepath.Join(configDir, "whitelist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载白名单失败：%w", err)
	}

	yellowList, err := r.loadConfig(filepath.Join(configDir, "yellowlist.yaml"))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载黄名单失败：%w", err)
	}

	return blackList, whiteList, yellowList, nil
}

func (r *securityConfigRepositoryImpl) loadConfig(filePath string) (*domain.Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.Config{Rules: []domain.Rule{}}, nil
		}
		return nil, fmt.Errorf("读取配置文件 [%s] 失败：%w", filePath, err)
	}

	var config domain.Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("解析 YAML 配置 [%s] 失败：%w", filePath, err)
	}

	return &config, nil
}
