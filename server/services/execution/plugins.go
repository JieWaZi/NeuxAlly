package execution

import (
	"context"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/authorization"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/packagefiles"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/runtimes"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// preparedPlugins 是本轮冻结的能力投射，不创建独立 Skill/MCP 资源。
type preparedPlugins struct {
	// skills 是可按需读取的原始插件技能。
	skills []runtimes.Skill
	// connectors 是经过配置和传输能力检查的派生连接器。
	connectors []models.MCPConnector
	// environment 是官方 CLI 所需的显式共享 profile 和私有程序目录。
	environment map[string]string
	// snapshots 记录实际交付组件与版本事实。
	snapshots []plugins.Snapshot
	// selected 记录本轮显式引用的插件快照。
	selected []plugins.Snapshot
	// release 在失败或执行结束时释放共享身份租约。
	release func()
}

// preparePlugins 在接纳前冻结资源，不向 Runtime 传递未就绪组件。
func (m *service) preparePlugins(ctx context.Context, waker string, request pendingMessage, detection runtimes.DetectionResult, usage ...authorization.Usage) (preparedPlugins, error) {
	result := preparedPlugins{environment: map[string]string{}, release: func() {}}
	selected := map[string]bool{}
	for _, id := range request.message.PluginIDs {
		selected[id] = true
	}
	if m.pluginController == nil {
		if len(selected) > 0 || len(request.pluginSnapshots) > 0 {
			return result, status_error.PluginNotReady
		}
		return result, nil
	}
	states, err := m.pluginController.InstalledStates(ctx, waker)
	if err != nil {
		return result, err
	}
	wanted := map[string]plugins.Snapshot{}
	for _, item := range request.pluginSnapshots {
		wanted[item.ID] = item
	}
	// 每个提供者在本轮中只验证一次，凭据始终留在当前内存投射。
	providerAccounts := map[string]authorization.Account{}
	providerErrors := map[string]error{}
	providerChecked := map[string]bool{}
	providerHeaders := map[string]map[string]string{}
	providerReleases := map[string]func(){}
	providerUsed := map[string]bool{}
	found := map[string]bool{}
	releases := []func(){}
	var once sync.Once
	result.release = func() {
		once.Do(func() {
			for _, release := range releases {
				release()
			}
		})
	}
	succeeded := false
	defer func() {
		if !succeeded {
			result.release()
		}
	}()
	for _, state := range states {
		id := state.Plugin.ID
		prior, isRetryResource := wanted[id]
		if request.retry && !isRetryResource {
			continue
		}
		var version models.PluginVersion
		for _, installation := range state.Installations {
			if installation.WakerID != waker || !installation.Enabled {
				continue
			}
			for _, candidate := range state.Versions {
				if candidate.ID == installation.VersionID {
					version = candidate
				}
			}
		}
		if isRetryResource {
			if state.Configuration.Revision != prior.ConfigurationRevision {
				return result, status_error.PluginConfigurationConflict
			}
			if version.ID != prior.VersionID {
				return result, status_error.PluginVersionConflict
			}
		}
		if version.ID == "" {
			continue
		}
		required := selected[id] || isRetryResource
		secrets := map[string]string{}
		for _, secret := range state.Configuration.Secrets {
			if secret.Configured {
				secrets[secret.Name] = secret.Value
			}
		}
		readyErr := plugins.ValidateValues(version.Package, state.Configuration.Values, secrets)
		extension, extensionErr := plugins.ClientExtension(version.Package)
		if readyErr == nil {
			readyErr = extensionErr
		}
		if len(version.Package.Servers) > 0 && version.TrustedAt == 0 {
			readyErr = status_error.MCPConnectorTrustRequired
		}
		if extension.Provider == constants.PluginProviderFeishu && request.allowedConnectorIDs != nil {
			readyErr = status_error.PluginForbidden
		}

		if readyErr == nil {
			readyErr = plugins.ValidateProviderServers(extension, version.Package)
		}
		if readyErr != nil {
			if required {
				return result, readyErr
			}
			continue
		}
		root, pathErr := plugins.ContentPath(m.skillDataDirectory, version.ContentPath)
		if pathErr != nil {
			if required {
				return result, pathErr
			}
			continue
		}
		data := filepath.Join(m.skillDataDirectory, "plugins", "data", id)
		if err = os.MkdirAll(data, 0700); err != nil {
			return result, status_error.PluginStorageError
		}
		if m.pluginContents != nil {
			release, err := m.pluginContents.Acquire(root, data)
			if err != nil {
				return result, err
			}
			releases = append(releases, release)
		}
		if err := plugins.VerifyContent(root, version.Digest); err != nil {
			if required {
				return result, err
			}
			continue
		}
		values := map[string]string{}
		for _, field := range extension.Config {
			if !field.Secret {
				values[field.Key] = field.Default
			}
		}
		for key, value := range state.Configuration.Values {
			values[key] = value
		}
		for key, value := range secrets {
			values[key] = value
		}
		snapshot := plugins.Snapshot{ID: id, Name: state.Plugin.Name, VersionID: version.ID, Version: version.Version, ConfigurationRevision: state.Configuration.Revision, ComponentIDs: []string{}}

		if extension.Provider != "" {
			auth := m.pluginAuthorizers[extension.Provider]
			statusErr := error(nil)
			if auth == nil {
				statusErr = status_error.PluginNotReady
			} else {
				if !providerChecked[extension.Provider] {
					providerChecked[extension.Provider] = true
					retained, leaseErr := auth.Acquire(usage...)
					if leaseErr != nil {
						providerErrors[extension.Provider] = leaseErr
					} else {
						releases = append(releases, retained)
						providerReleases[extension.Provider] = retained
						providerAccounts[extension.Provider], providerErrors[extension.Provider] = auth.Status(ctx, true)
					}
				}
				account := providerAccounts[extension.Provider]
				statusErr = providerErrors[extension.Provider]
				if statusErr == nil && !account.Available {
					statusErr = status_error.PluginNotReady
				}
				for _, scope := range extension.Scopes {
					if !auth.ScopeGranted(account.Scopes, scope) {
						statusErr = status_error.PluginNotReady
					}
				}
				if statusErr == nil && isRetryResource && prior.ProviderAccountID != account.ID {
					statusErr = status_error.PluginConfigurationConflict
				}
			}
			if statusErr != nil {
				if required {
					return result, statusErr
				}
				continue
			}
		}
		candidateSkills := []runtimes.Skill{}
		candidateConnectors := []models.MCPConnector{}
		available := map[string]bool{}
		if detection.Capabilities.Has(runtimes.CapabilitySkillDirectory) {
			for _, skill := range version.Package.Skills {
				candidateSkills = append(candidateSkills, runtimes.Skill{Name: skill.Name, Path: filepath.Join(root, skill.Path)})
				available[skill.Key] = true
				snapshot.ComponentIDs = append(snapshot.ComponentIDs, plugins.ComponentID(id, "skill:"+skill.Key))
			}
		}
		for _, server := range version.Package.Servers {
			transport := constants.RuntimeConnectorTransport(server.Type)
			if server.Type == "streamable-http" {
				transport = constants.RuntimeConnectorTransportStreamableHTTP
			}
			componentID := plugins.ComponentID(id, "mcp:"+server.Key)
			if request.allowedConnectorIDs != nil && !slices.Contains(*request.allowedConnectorIDs, componentID) {
				continue
			}
			if !runtimes.DetectedConnectorTransportSupported(detection, transport) {
				continue
			}
			resolved, resolveErr := plugins.ResolveServer(version.Package, server, root, data, values)
			if resolveErr != nil {
				if required {
					return result, resolveErr
				}
				continue
			}
			connector := models.MCPConnector{ID: componentID, Name: server.Key, RuntimeKey: "ally_plugin_" + componentID, Transport: constants.MCPTransport(transport), Command: resolved.Server.Command, Arguments: resolved.Server.Args, URL: resolved.Server.URL, TimeoutMS: 30000}
			if transport == constants.RuntimeConnectorTransportStdio {
				if m.pluginLauncher == "" {
					if required {
						return result, status_error.PluginNotReady
					}
					continue
				}
				connector.Command = m.pluginLauncher
				connector.Arguments = append([]string{constants.PluginStdioMode, "--cwd", resolved.WorkingDirectory, "--", resolved.Server.Command}, resolved.Server.Args...)
			}
			if transport == constants.RuntimeConnectorTransportStdio {
				for name, value := range resolved.Server.Env {
					connector.Environment = append(connector.Environment, models.MCPSecretDefinition{Name: name, Value: value, Configured: true})
				}
			}

			if extension.Provider == constants.PluginProviderGitHub {
				headers, checked := providerHeaders[extension.Provider]
				if !checked {
					headers, err = m.pluginAuthorizers[extension.Provider].AuthorizeServer(ctx, resolved.Server.URL)
					if err != nil {
						if required {
							return result, err
						}
						continue
					}
					providerHeaders[extension.Provider] = headers
				}
				if resolved.Server.Headers == nil {
					resolved.Server.Headers = map[string]string{}
				}
				for key := range resolved.Server.Headers {
					if strings.EqualFold(key, "Authorization") || strings.HasPrefix(strings.ToLower(key), "x-mcp-") {
						delete(resolved.Server.Headers, key)
					}
				}
				for key, value := range headers {
					resolved.Server.Headers[key] = value
				}
				resolved.Server.Headers["X-MCP-Toolsets"] = constants.GitHubMCPToolsets
			}
			for name, value := range resolved.Server.Headers {
				connector.Headers = append(connector.Headers, models.MCPSecretDefinition{Name: name, Value: value, Configured: true})
			}
			candidateConnectors = append(candidateConnectors, connector)
			available[server.Key] = true
			snapshot.ComponentIDs = append(snapshot.ComponentIDs, componentID)
		}
		missingRequired := false
		for _, key := range extension.RequiredComponents {
			if !available[key] {
				missingRequired = true
			}
		}
		if missingRequired || len(snapshot.ComponentIDs) == 0 {
			if required {
				return result, status_error.PluginNotReady
			}
			continue
		}
		if isRetryResource && !slices.Equal(snapshot.ComponentIDs, prior.ComponentIDs) {
			return result, status_error.PluginNotReady
		}

		if extension.Provider != "" {
			auth := m.pluginAuthorizers[extension.Provider]
			account := providerAccounts[extension.Provider]
			snapshot.ProviderAccountID = account.ID
			providerUsed[extension.Provider] = true
			if extension.Provider != constants.PluginProviderGitHub {
				for key, value := range auth.Environment() {
					result.environment[key] = value
				}
			}
			result.environment["NEUXALLY_"+strings.ToUpper(extension.Provider)+"_IDENTITY"] = account.AppID + ":" + account.ID
			if extension.Provider != constants.PluginProviderGitHub {
				directory := filepath.Dir(auth.Binary())
				if result.environment["PATH"] == "" {
					result.environment["PATH"] = directory
				} else if !slices.Contains(filepath.SplitList(result.environment["PATH"]), directory) {
					result.environment["PATH"] += string(os.PathListSeparator) + directory
				}
			}
		}

		result.skills = append(result.skills, candidateSkills...)
		result.connectors = append(result.connectors, candidateConnectors...)
		result.snapshots = append(result.snapshots, snapshot)
		if selected[id] {
			result.selected = append(result.selected, snapshot)
		}
		found[id] = true
	}
	for id := range selected {
		if !found[id] {
			return result, status_error.PluginNotReady
		}
	}
	for id := range wanted {
		if !found[id] {
			return result, status_error.PluginNotReady
		}
	}
	for provider, release := range providerReleases {
		if !providerUsed[provider] {
			release()
		}
	}
	succeeded = true
	return result, nil
}

// mergePluginSkills 只在同名且完整内容相同的情况下共用一份投射。
func mergePluginSkills(independent, derived []runtimes.Skill) ([]runtimes.Skill, error) {
	result := make([]runtimes.Skill, 0, len(independent)+len(derived))
	indexes := map[string]int{}
	digests := map[string]string{}
	policy := packagefiles.Archive{Invalid: status_error.PluginContentInvalid, Limit: status_error.PluginContentLimitExceeded, IO: status_error.PluginStorageError}
	for _, item := range append(append([]runtimes.Skill{}, independent...), derived...) {
		existing, ok := indexes[item.Name]
		if !ok {
			indexes[item.Name] = len(result)
			result = append(result, item)
			continue
		}
		digest, err := policy.Digest(item.Path)
		if err != nil {
			return nil, err
		}
		previous := digests[result[existing].Path]
		if previous == "" {
			previous, err = policy.Digest(result[existing].Path)
			if err != nil {
				return nil, err
			}
			digests[result[existing].Path] = previous
		}
		if previous != digest {
			return nil, status_error.PluginSkillConflict
		}
		result[existing].Selected = result[existing].Selected || item.Selected
		// 插件内容已持有执行租约，去重后继续使用受保护的不可变目录。
		result[existing].Path = item.Path
	}
	return result, nil
}

// pluginSelectionHint 将明确插件意图附在本轮消息，保留内部组件的按需发现语义。
func pluginSelectionHint(selected []plugins.Snapshot) string {
	if len(selected) == 0 {
		return ""
	}
	names := make([]string, 0, len(selected))
	for _, item := range selected {
		names = append(names, item.Name+" ("+item.ID+")")
	}
	return "\n\n用户本轮明确选择插件：" + strings.Join(names, "、") + "。请按任务需要使用其可用技能或连接器。"
}
