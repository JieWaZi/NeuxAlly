package core

import (
	"context"
	"fmt"
	types "github.com/JieWaZi/neux-knowledge/internal/domain"
	"github.com/JieWaZi/neux-knowledge/internal/pipeline"
	"github.com/JieWaZi/neux-knowledge/internal/wiki"
)

// 以下别名公开知识阅读和直接模型端口。
type (
	ReadRequest      = pipeline.ReadRequest
	ReadArtifact     = pipeline.ReadArtifact
	ReadBlock        = pipeline.Chunk
	Chunk            = pipeline.Chunk
	ReaderPort       = pipeline.ReaderPort
	MarkdownReader   = pipeline.MarkdownReader
	ModelMessage     = pipeline.ModelMessage
	ModelUsage       = types.ModelUsage
	ModelDiagnostics = types.ModelDiagnostics
	GenerateRequest  = pipeline.GenerateRequest
	GenerateResult   = pipeline.GenerateResult
	Generator        = pipeline.Generator
	OrganizeInput    = pipeline.Input
	OrganizeResult   = pipeline.Result
)

// RunOrganize 执行固定 Lite 管线，模型调用不创建聊天或通用执行。
func (e *Engine) RunOrganize(ctx context.Context, in OrganizeInput) (*OrganizeResult, error) {
	return pipeline.Run(ctx, e.store, in)
}

// CancelOrganize 仅供删除、授权撤销等内部生命周期立取消屏障；调用方同时取消运行Context，不公开用户停止接口。
func (e *Engine) CancelOrganize(ctx context.Context, in JobActionInput) (*Receipt, error) {
	rec, err := e.store.CancelJob(ctx, in)
	if err != nil {
		return nil, err
	}
	job, err := e.store.GetJob(ctx, in.JobID)
	if err != nil {
		return nil, err
	}
	release, acquired, err := e.store.LockProcessor(ctx, "lite/library/"+job.LibraryID, 1)
	if err != nil {
		return nil, err
	}
	if acquired {
		defer release()
		if err = e.store.RecoverLiteStages(ctx, job.ID, job.Attempt); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

// OrganizeConfigDigest 固定库配置、资源和 Lite 提示词版本。
func OrganizeConfigDigest(lib *Library, rulesSHA256 string) string {
	return types.OrganizeConfigDigest(lib, rulesSHA256, wiki.StagePromptVersion)
}

// FailOrganize 终结本次不可恢复故障，保留已发布页面与成功来源；进程退出取消不得调用。
func (e *Engine) FailOrganize(ctx context.Context, jobID string, attempt int, diagnostic string) (*Receipt, error) {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.Attempt != attempt {
		return nil, &types.Error{Code: types.ErrStaleAttempt, Operation: "lite.fail", Detail: "job attempt changed"}
	}
	sources := []types.SourceConclusion{}
	for _, id := range job.SourceVersionIDs {
		source, err := e.GetSource(ctx, job.LibraryID, id)
		if err != nil {
			return nil, err
		}
		if source.State != "processed" && source.State != "no-change" {
			sources = append(sources, types.SourceConclusion{SourceVersionID: id, State: "failed", Diagnostic: diagnostic})
		}
	}
	return e.store.FailJob(ctx, types.FailJobInput{JobActionInput: types.JobActionInput{RequestID: fmt.Sprintf("lite/%s/%d/fatal", jobID, attempt), JobID: jobID, ExpectedVersion: job.Version}, Diagnostic: diagnostic, Sources: sources})
}
