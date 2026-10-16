package role_presets

import "github.com/shrewx/ginx"

// Router 是角色预设接口路由。
var Router = ginx.NewRouter(ginx.Group("/role-presets"))

func init() {
	Router.Register(&ListRolePresets{})
	Router.Register(&CreateRolePreset{})
	Router.Register(&ExportProfileTemplate{})
	Router.Register(&GetRolePreset{})
	Router.Register(&UpdateRolePreset{})
	Router.Register(&DeleteRolePreset{})
	Router.Register(&ListRolePresetDocuments{})
	Router.Register(&ExportRolePreset{})
}
