package knowledge_connections

import (
	"context"
	"errors"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/remote"
)

// remoteClients 仅包含远端HTTP适配器，不能访问本机知识引擎或Runtime。
type remoteClients struct {
	// connection 为固定连接配置。
	connection models.KnowledgeConnection
	// official 为官方协议分支。
	official *remote.WeKnora
	// custom 为自有协议分支。
	custom *remote.Custom
}

func client(ctx context.Context, connectionID, libraryID string) (*remoteClients, error) {
	if global.KnowledgeConnectionControllerManager == nil {
		return nil, status_error.KnowledgeUnsupported
	}
	connection, token, err := global.KnowledgeConnectionControllerManager.Snapshot(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if !connection.Enabled {
		return nil, status_error.KnowledgeForbidden
	}
	out := &remoteClients{connection: connection}
	options := remote.Options{BaseURL: connection.BaseURL, Token: token, AuthMode: connection.AuthMode}
	switch connection.Backend {
	case "weknora":
		out.official, err = remote.NewWeKnora(options)
	case "custom":
		out.custom, err = remote.NewCustom(options, remote.ResourceRef{Backend: "custom", ConnectionID: connectionID, KnowledgeBaseID: libraryID})
	default:
		return nil, status_error.KnowledgeUnsupported
	}
	return out, remoteError(err)
}
func remoteError(err error) error {
	if err == nil {
		return nil
	}
	var e *remote.Error
	if errors.As(err, &e) {
		switch e.Code {
		case "unauthenticated":
			return status_error.KnowledgeRemoteUnauthorized
		case "forbidden", "scope_violation", "policy_changed", "write_ip_forbidden":
			return status_error.KnowledgeForbidden
		case "not_found":
			return status_error.KnowledgeNotFound
		case "version_conflict", "idempotency_conflict", "manual_protected":
			return status_error.KnowledgeConflict
		case "unsupported", "unsupported_profile", "unsupported_source_state":
			return status_error.KnowledgeUnsupported
		case "invalid", "invalid_connection", "invalid_resource", "invalid_request", "invalid_query", "invalid_pagination":
			return status_error.KnowledgeInvalid
		case "cancelled", "stale_attempt":
			return status_error.KnowledgeCancelled
		case "candidate_invalid", "execution_not_successful":
			return status_error.KnowledgeCandidateInvalid
		case "protocol_incompatible", "redirect_rejected", "response_too_large", "response_incomplete":
			return status_error.KnowledgeRemoteIncompatible
		case "outcome_unknown":
			return status_error.KnowledgeRemoteOutcomeUnknown
		}
	}
	return status_error.KnowledgeRemoteUnreachable
}
