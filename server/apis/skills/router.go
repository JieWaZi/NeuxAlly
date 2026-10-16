// Package skills 提供伙伴技能查询、导入和启停接口。
package skills

import "github.com/shrewx/ginx"

// Router 是伙伴技能接口路由。
var Router = ginx.NewRouter(ginx.Group("/wakers"))

func init() {
	Router.Register(&ListWakerSkills{})
	Router.Register(&GetWakerSkill{})
	Router.Register(&UpdateWakerSkill{})
	Router.Register(&UninstallSkill{})
	Router.Register(&ListSkillVersions{})
	Router.Register(&CreateSkillVersion{})
	Router.Register(&GetSkillVersion{})
	Router.Register(&GetSkillVersionFile{})
	Router.Register(&RestoreSkillVersion{})
	Router.Register(&PullSkillUpdate{})
	Router.Register(&SetWakerSkillEnabled{})
	Router.Register(&ScanWakerSkillImport{})
	Router.Register(&ImportWakerSkill{})
}
