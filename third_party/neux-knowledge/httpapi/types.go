// Package httpapi 提供只包含知识用例的可嵌入 HTTP 边界，不暴露宿主管理 API。
package httpapi

import (
	"context"
	"github.com/JieWaZi/neux-knowledge/core"
)

const BasePath = "/api/knowledge/v1"

// Authorization 固定接纳身份，并为实际发布提供可重复获得的撤销闸门。
type Authorization struct {
	Subject      string                                // 认证主体，不是客户端自报身份。
	Permission   core.PermissionSnapshot               // 服务器验证的真实库范围和权限代次。
	PublishGuard func(context.Context) (func(), error) // 持读锁复核策略；发布事务结束后必须释放。
	Recovery     RecoveryIdentity                      // 可持久保存的原始授权身份，不包含令牌正文。
}

// RecoveryIdentity 仅由宿主保存，恢复任务不接受客户端提交此身份。
type RecoveryIdentity struct {
	Subject          string                  `json:"subject"`          // 接纳时认证主体。
	CredentialSHA256 string                  `json:"credentialSHA256"` // 原凭据摘要，仅用于验证同一凭据仍有效。
	LibraryID        string                  `json:"libraryId"`        // 冻结的库目标。
	Operation        string                  `json:"operation"`        // 原始获准操作。
	RemoteAddr       string                  `json:"remoteAddr"`       // 原始真实 TCP 对端。
	ForwardedFor     string                  `json:"forwardedFor"`     // 原请求转发链，恢复时按当前可信代理规则重核。
	Permission       core.PermissionSnapshot `json:"permission"`       // 接纳时范围和策略代次。
}

// OrganizeRequest 只选择服务端预配 profile，不接收本机账号、CLI 或代码路径。
type OrganizeRequest struct {
	RequestID        string   `json:"requestId"`             // 原始幂等请求身份。
	LibraryID        string   `json:"libraryId"`             // 目标知识库。
	ExpectedVersion  int64    `json:"expectedVersion"`       // 当前库版本。
	StructureVersion int64    `json:"structureVersion"`      // 当前目录代次。
	SourceVersionIDs []string `json:"sourceVersionIds"`      // 明确冻结的原件版本。
	ProfileID        string   `json:"profileId"`             // 服务端预配执行配置身份。
	Rebuild          bool     `json:"rebuild,omitempty"`     // 显式重建所选原件，省略时保留普通整理语义。
	Instruction      string   `json:"instruction,omitempty"` // 本次整理要求，不改变服务端执行身份与授权范围。
}

// MaintenanceRequest 交给部署端真实会话用例，HTTP 层不启动第二模型宿主。
type MaintenanceRequest struct {
	RequestID           string `json:"requestId"`           // 原始幂等请求身份。
	LibraryID           string `json:"libraryId"`           // 目标知识库。
	ExpectedVersion     int64  `json:"expectedVersion"`     // 当前库版本。
	StructureVersion    int64  `json:"structureVersion"`    // 当前目录代次。
	ProfileID           string `json:"profileId"`           // 服务端预配配置身份。
	Instruction         string `json:"instruction"`         // 用户维护指令，不作为授权或路径。
	PageID              string `json:"pageId"`              // 明确冻结的维护目标页面。
	ExpectedPageVersion int    `json:"expectedPageVersion"` // 用户已读取的页面版本。
}

// Profile 只公布服务器可选择的配置身份，不包含执行账号或路径。
type Profile struct {
	ID   string `json:"id"`   // 服务端固定配置身份。
	Name string `json:"name"` // 用户可见名称。
}

// ExecutionPort 由已存在的宿主业务服务装配；必须把 PublishGuard 传入核心管线。
type ExecutionPort interface {
	Organize(context.Context, OrganizeRequest, Authorization) (*core.Receipt, error)
	Maintain(context.Context, MaintenanceRequest, Authorization) (*core.Receipt, error)
}

// Options 声明独立认证和真实执行端口；缺省认证拒绝所有请求。
type Options struct {
	Security  *PolicyStore  // 所有入口和发布闸门共享的动态策略存储。
	Execution ExecutionPort // 未装配时整理和维护准确返回 unsupported。
	Profiles  []Profile     // 服务器预配且允许远端选择的配置，不包含秘密。
}

// APIError 只向调用方返回稳定业务码，不传播底层文件路径或凭据。
type APIError struct {
	Code   string `json:"code"` // 稳定机器错误码。
	Status int    `json:"-"`    // HTTP 状态。
}

func (e *APIError) Error() string { return e.Code }

// List 返回明确的实际分页事实。
type List[T any] struct {
	Items  []T   `json:"items"`  // 本页实际记录。
	Total  int64 `json:"total"`  // 授权范围中的实际总量。
	Offset int   `json:"offset"` // 起始偏移。
	Limit  int   `json:"limit"`  // 有界分页上限。
}

// Capabilities 区分协议支持、当前授权及执行装配事实。
type Capabilities struct {
	Profile    string                      `json:"profile"`    // 自有固定版本合同。
	Operations map[string]bool             `json:"operations"` // 当前请求实际可用的动作。
	Profiles   []Profile                   `json:"profiles"`   // 当前可选择的服务端配置。
	Details    map[string]CapabilityStatus `json:"details"`    // 分开说明协议支持、凭据权限和当前资源条件。
}

// CapabilityStatus 避免把协议支持当作当前凭据或资源已经获准。
type CapabilityStatus struct {
	Supported  bool   `json:"supported"`  // 当前服务实际装配此用例。
	Permission string `json:"permission"` // allowed 或 denied，逐请求计算。
	Condition  string `json:"condition"`  // ready 或明确的策略／执行配置限制。
}
