package plugininstallation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcommand"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins"
	"github.com/JieWaZi/NeuxAlly/server/internal/plugins/catalog"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
)

// AgentBindings 只绑定当前Execution与伙伴；输入不能指定用户CLI作用域。
func (s *Service) AgentBindings(chat models.Chat, executionID string) []agentcapability.Binding {
	if chat.WakerID == "" || executionID == "" {
		return nil
	}
	scope := "plugin-install:" + chat.WakerID + ":" + executionID
	bindings := []agentcapability.Binding{}
	for _, operation := range []string{"plugins.catalog.search", "plugins.catalog.get", "plugins.installations.install", "plugins.installations.get"} {
		id := operation
		binding := agentcapability.Binding{Operation: id, Call: func(ctx context.Context, request agentcapability.Request) (agentcapability.Result, error) {
			switch id {
			case "plugins.catalog.search":
				entries, err := catalog.List(s.config.CatalogSource)
				if err != nil {
					return agentcapability.Result{}, err
				}
				query, _ := request.Params["query"].(string)
				query = strings.ToLower(strings.TrimSpace(query))
				items := []catalog.Entry{}
				for _, entry := range entries {
					if query == "" || strings.Contains(strings.ToLower(entry.Package.Manifest.Name+" "+entry.Package.Manifest.Description), query) {
						items = append(items, entry)
					}
				}
				return agentcapability.Result{Data: map[string]any{"items": items}}, nil
			case "plugins.catalog.get":
				key, _ := request.Params["key"].(string)
				entry, err := catalog.Find(s.config.CatalogSource, key)
				return agentcapability.Result{Data: map[string]any{"entry": entry}}, err
			case "plugins.installations.get":
				pluginID, _ := request.Params["pluginId"].(string)
				resource, err := s.Visible(ctx, pluginID, chat.WakerID)
				if err != nil {
					return agentcapability.Result{}, err
				}
				relations, err := s.config.Store.Installations(ctx, resource.ID, chat.WakerID)
				if err != nil {
					return agentcapability.Result{}, err
				}
				data := map[string]any{"pluginId": resource.ID, "installations": relations, "ready": false, "state": "not-installed"}
				if len(relations) > 0 {
					version, err := s.config.Store.Version(ctx, relations[0].VersionID)
					if err != nil {
						return agentcapability.Result{}, err
					}
					config, err := s.config.Store.Configuration(ctx, resource.ID)
					if err != nil {
						return agentcapability.Result{}, err
					}
					account, readyErr := s.Readiness(ctx, version, config, true)
					data["ready"] = readyErr == nil
					data["account"] = account
					data["state"] = "installed"
				}
				return agentcapability.Result{Data: data}, nil
			default:
				return agentcommand.Execute(ctx, s.config.Commands, scope, request, func(ctx context.Context) error { return s.installCatalog(ctx, chat.WakerID, request) })
			}
		}}
		if id == "plugins.installations.install" {
			binding.Receipt = func(ctx context.Context, key string) (agentcapability.Result, error) {
				return agentcommand.Lookup(ctx, s.config.Commands, scope, id, key)
			}
		}
		bindings = append(bindings, binding)
	}
	return bindings
}

func (s *Service) installCatalog(ctx context.Context, waker string, request agentcapability.Request) error {
	key, _ := request.Params["key"].(string)
	trusted, _ := request.Params["trustConfirmed"].(bool)
	entry, err := catalog.Find(s.config.CatalogSource, key)
	if err != nil {
		return err
	}
	location, err := catalog.ResolveEntry(s.config.CatalogSource, entry)
	if err != nil {
		return err
	}
	parent := filepath.Join(s.config.DataDirectory, "plugins", "staging")
	stage, err := plugins.StageSource(parent, location, entry.PackagePath)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(parent, stage.Token))
	resource, versionID, err := s.Prepare(ctx, stage, "", "", trusted, entry.SourceKey, entry.SourceCommit, entry.ArchiveSHA256, entry.ArchiveURL)
	if err != nil {
		return err
	}
	version, err := s.config.Store.Version(ctx, versionID)
	if err != nil {
		return err
	}
	config, err := s.config.Store.Configuration(ctx, resource.ID)
	if err != nil {
		return err
	}
	if _, err := s.Readiness(ctx, version, config, true); err != nil {
		if !errors.Is(err, status_error.PluginNotReady) || version.Package.Integration.Authentication == nil {
			return err
		}
		auth, _, err := s.Bridge(version)
		if err != nil {
			return err
		}
		session, err := auth.Login(ctx, version.Package.Integration.Authentication.Scopes...)
		if err != nil {
			return err
		}
		return s.config.Transactions.Transaction(ctx, func(txctx context.Context) error {
			return agentcommand.Commit(txctx, map[string]any{"pluginId": resource.ID, "versionId": version.ID, "installed": false, "state": "authorization-required", "sessionId": session.ID, "nextStep": "Complete authorization in the plugin interface, then call installation again with a new request ID."})
		})
	}
	results, err := s.Install(ctx, resource.ID, versionID, config.Revision, []string{waker})
	if err != nil {
		return err
	}
	if len(results) != 1 || !results[0].Installed {
		return status_error.PluginNotReady
	}
	return nil
}
