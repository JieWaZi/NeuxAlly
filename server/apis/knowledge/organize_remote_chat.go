package knowledge

import (
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	bridge "github.com/JieWaZi/NeuxAlly/server/internal/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/internal/workspace"
	service "github.com/JieWaZi/NeuxAlly/server/services/knowledge"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 整理聊天所选附件到远端；不会创建本机模型执行。
type OrganizeRemoteChat struct {
	ginx.MethodPost
	// 聊天标识
	ChatID string `in:"path" name:"chatId" validate:"required,max=32"`
	// 请求体
	Body RemoteChatInput `in:"body"`
}

func (o *OrganizeRemoteChat) Path() string { return "/chat-operations/:chatId" }
func (o *OrganizeRemoteChat) Output(ctx *gin.Context) (interface{}, error) {
	if global.KnowledgeService == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	attachments := make([]workspace.AttachmentInput, 0, len(o.Body.Attachments))
	for _, attachment := range o.Body.Attachments {
		attachments = append(attachments, workspace.AttachmentInput{SourcePath: attachment.SourcePath})
	}
	result, err := global.KnowledgeService.OrganizeRemoteChat(ctx, service.RemoteChatCommand{RequestID: o.Body.RequestID, ChatID: o.ChatID, Ref: bridge.ResourceRef{Backend: o.Body.Ref.Backend, ConnectionID: o.Body.Ref.ConnectionID, KnowledgeBaseID: o.Body.Ref.KnowledgeBaseID}, ProfileID: o.Body.ProfileID, Attachments: attachments})
	if err != nil {
		return nil, err
	}
	return remoteChatDTO(result), nil
}
func init() { Router.Register(&OrganizeRemoteChat{}) }
