package knowledge_connections

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/JieWaZi/NeuxAlly/server/apis/knowledge"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/core"
	"github.com/gin-gonic/gin"
)

// remoteInvoke 只让首次持久接纳的请求发出写入；重启或并发重放均只查询原回执。
func remoteInvoke(ctx *gin.Context, c *remoteClients, requestID, operation string, input any, call func() (*core.Receipt, error)) (knowledge.Receipt, error) {
	if global.KnowledgeControllerManager == nil {
		return knowledge.Receipt{}, status_error.KnowledgeUnsupported
	}
	// 只存摘要与完整目标；原件、正文、凭据和维护要求不进入主库。
	canonical, err := json.Marshal(struct {
		// Ref 固定完整远端目标。
		Ref knowledge.ResourceRef
		// ConnectionRevision 固定连接配置，防止重配后向另一服务器对账。
		ConnectionRevision int64
		// Operation 固定真实远端动作。
		Operation string
		// Input 仅在内存中计算规范化摘要。
		Input any
	}{remoteRef(c, c.custom.Resource().KnowledgeBaseID), c.connection.Revision, operation, input})
	if err != nil {
		return knowledge.Receipt{}, status_error.KnowledgeInvalid
	}
	sum := sha256.Sum256(canonical)
	link := models.KnowledgeOperationLink{RequestID: requestID, Operation: operation, Digest: hex.EncodeToString(sum[:]), Backend: c.connection.Backend, ConnectionID: c.connection.ID, KnowledgeBaseID: c.custom.Resource().KnowledgeBaseID, ConnectionRevision: c.connection.Revision}
	fresh, err := global.KnowledgeControllerManager.ClaimRemoteOperation(ctx, link)
	if err != nil {
		return knowledge.Receipt{}, err
	}
	if !fresh {
		return reconcileRemote(ctx, c, requestID)
	}
	receipt, err := call()
	if err != nil {
		mapped := remoteError(err)
		if !errors.Is(mapped, status_error.KnowledgeRemoteOutcomeUnknown) {
			var stable interface {
				error
				Code() int64
			}
			if errors.As(mapped, &stable) {
				if persist := global.KnowledgeControllerManager.RejectRemoteOperation(ctx, requestID, stable.Code()); persist != nil {
					return knowledge.Receipt{}, persist
				}
			}
		}
		return knowledge.Receipt{}, mapped
	}
	return remoteWriteResult(ctx, c, receipt, nil)
}

// reconcileRemote 从原服务器查询同一请求；回执读取失败不证明原写入被拒绝，不会重新 POST。
func reconcileRemote(ctx *gin.Context, c *remoteClients, requestID string) (knowledge.Receipt, error) {
	if global.KnowledgeControllerManager == nil {
		return knowledge.Receipt{}, status_error.KnowledgeUnsupported
	}
	link, err := global.KnowledgeControllerManager.GetOperation(ctx, requestID)
	if err != nil {
		return knowledge.Receipt{}, err
	}
	target := c.custom.Resource()
	if link.Backend != c.connection.Backend || link.ConnectionID != c.connection.ID || link.ConnectionRevision != c.connection.Revision || (target.KnowledgeBaseID != "" && link.KnowledgeBaseID != target.KnowledgeBaseID) {
		return knowledge.Receipt{}, status_error.KnowledgeConflict
	}
	if link.RemoteErrorCode != 0 {
		return knowledge.Receipt{}, status_error.StatusError(link.RemoteErrorCode)
	}
	if link.Operation == "library.create" {
		// 新库身份可能尚未返回；即使已恢复也不能假定凭据自动获得新库 grant。
		receipt, err := c.custom.GetCreationReceipt(ctx, requestID)
		if err != nil {
			// 当前读取授权、网络或协议失败均不能改写已发送创建的结果。
			return knowledge.Receipt{}, status_error.KnowledgeRemoteOutcomeUnknown
		}
		if link.KnowledgeBaseID != "" && receipt.LibraryID != link.KnowledgeBaseID {
			return knowledge.Receipt{}, status_error.KnowledgeRemoteOutcomeUnknown
		}
		return remoteWriteResult(ctx, c, receipt, nil)
	}
	if link.KnowledgeBaseID == "" {
		return knowledge.Receipt{}, status_error.KnowledgeRemoteOutcomeUnknown
	}
	scoped := c
	if target.KnowledgeBaseID == "" {
		scoped, err = client(ctx, c.connection.ID, link.KnowledgeBaseID)
		if err != nil {
			return knowledge.Receipt{}, status_error.KnowledgeRemoteOutcomeUnknown
		}
	}
	receipt, err := scoped.custom.GetReceipt(ctx, requestID)
	if err != nil {
		// 只有首次写入已持久的 RemoteErrorCode 能证明明确拒绝；读取失败保留原身份。
		return knowledge.Receipt{}, status_error.KnowledgeRemoteOutcomeUnknown
	}
	return remoteWriteResult(ctx, scoped, receipt, nil)
}
