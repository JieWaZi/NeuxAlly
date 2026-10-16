package plugins

import (
	"context"
	"maps"
	"slices"

	"github.com/JieWaZi/NeuxAlly/server/global"
	packageplugin "github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// prepareInstallationVersions 为首次安装和版本切换复用内容、依赖与认证预检。
// 文件和官方 CLI 在事务外读取；返回的占用必须在数据库提交后释放。
func prepareInstallationVersions(ctx context.Context, versions []models.PluginVersion, config models.PluginConfiguration) (func(), error) {
	releases := []func(){}
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	prepared := false
	defer func() {
		if !prepared {
			release()
		}
	}()
	providers := map[string]bool{}
	for _, version := range versions {
		root, err := packageplugin.ContentPath(global.Config.DataDirectory, version.ContentPath)
		if err != nil {
			return nil, err
		}
		if global.PluginContents != nil {
			retained, err := global.PluginContents.Acquire(root)
			if err != nil {
				return nil, err
			}
			releases = append(releases, retained)
		}
		if err := packageplugin.VerifyContent(root, version.Digest); err != nil {
			return nil, err
		}
		extension, err := packageplugin.ClientExtension(version.Package)
		if err != nil {
			return nil, err
		}
		if extension.Provider != "" {
			providers[extension.Provider] = true
		}
	}
	for provider := range providers {
		if auth := global.PluginAuthorizations[provider]; auth != nil {
			retained, err := auth.Acquire()
			if err != nil {
				return nil, err
			}
			releases = append(releases, retained)
			account, err := auth.Status(ctx, true)
			if err != nil {
				return nil, err
			}
			ctx = context.WithValue(ctx, providerSnapshotKey{Provider: provider}, providerSnapshot{Account: account})
		}
	}
	for _, version := range versions {
		if _, err := readiness(ctx, version, config, false); err != nil {
			return nil, err
		}
	}
	prepared = true
	return release, nil
}

// configurationChange 生成请求后的独立配置副本，预检和事务使用同一规则。
func configurationChange(config models.PluginConfiguration, input UpdateConfigurationRequest) models.PluginConfiguration {
	config.Values = maps.Clone(config.Values)
	config.Secrets = slices.Clone(config.Secrets)
	if input.Values != nil {
		config.Values = maps.Clone(input.Values)
	}
	for _, secret := range input.Secrets {
		index := slices.IndexFunc(config.Secrets, func(value models.MCPSecretDefinition) bool { return value.Name == secret.Key })
		if index >= 0 {
			if secret.Clear {
				config.Secrets = append(config.Secrets[:index], config.Secrets[index+1:]...)
			} else if secret.Value != "" {
				config.Secrets[index].Value = secret.Value
				config.Secrets[index].Configured = true
			}
		} else if !secret.Clear && secret.Value != "" {
			config.Secrets = append(config.Secrets, models.MCPSecretDefinition{ID: secret.Key, Name: secret.Key, Configured: true, Value: secret.Value})
		}
	}
	return config
}
