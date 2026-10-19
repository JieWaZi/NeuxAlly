package contract

import "strings"

// outputSchema 只维护 CLI 的结果边界，不复制源码 Manifest、运行证据或动态用户 Schema。
func outputSchema(id string) map[string]any {
	object := func(names ...string) map[string]any {
		properties := map[string]any{}
		for _, name := range names {
			properties[name] = map[string]any{}
		}
		return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	}
	if strings.HasSuffix(id, ".wait") {
		return object("completed", "resource")
	}
	switch id {
	case "workflow.authoring.prepare":
		return object("workflow", "annotations", "topics", "guideIds", "content", "resources", "operations")
	case "im.targets.list", "team.tasks.list", "workflow.annotations.list", "workflow.definitions.list":
		return object("items", "total", "pageNum", "pageSize", "nextCursor")
	case "im.deliveries.create", "im.deliveries.get":
		return object("deliveryId", "state", "errorCode", "partsSent", "createdAt")
	case "workflow.source.get":
		return object("workflowId", "revisionId", "currentRevisionId", "source", "sourceDigest", "sdkVersion")
	case "workflow.source.validate":
		return object("valid", "diagnostics", "summary", "manifest", "error")
	case "workflow.source.save":
		return object("saved", "definitionId", "revisionId", "ordinal", "sourceDigest", "unchanged", "replayed")
	case "workflow.sdk.get":
		return object("version", "topic", "content", "source", "guidance")
	case "workflow.resources.list":
		return object("kind", "items", "nextCursor", "toolPolicyCapabilities")
	case "workflow.definitions.describe":
		return object("id", "name", "description", "whenToUse", "revisionId", "runnable", "inputSchema", "outputSchema")
	case "workflow.runs.start":
		return object("runId", "definitionId", "revisionId")
	case "workflow.runs.cancel":
		return object("runId", "definitionId", "cancellationRequested")
	case "workflow.runs.get":
		return object("run", "humanRequests", "operationRequests")
	case "workflow.evidence.list":
		return object("run", "instances", "actionCalls", "humanRequests", "operations", "totalInstances", "nextCursor")
	case "workflow.results.schema":
		return object("schema", "nodeId", "guidance")
	case "workflow.results.submit":
		return object("submitted", "executionId", "attemptGeneration")
	case "team.members.list":
		return object("items")
	case "team.tasks.get":
		return object("id", "threadId", "senderWakerId", "assigneeWakerId", "subject", "description", "status", "priority", "requiresHumanReview", "directDelivery", "currentFence", "result", "errorMessage", "changedPaths", "createdAt", "updatedAt", "startedAt", "finishedAt", "cancelRequestedAt")
	case "team.tasks.create":
		return object("taskId", "accepted")
	case "team.tasks.update":
		return object("taskId", "updated")
	case "team.tasks.follow-up":
		return object("taskId", "followUpId", "accepted")
	case "team.tasks.retry":
		return object("taskId", "retryQueued")
	case "team.tasks.cancel":
		return object("taskId", "cancellationRequested")
	case "team.reports.submit":
		return object("reported", "taskId", "attemptId", "nextStep")
	case "memory.entries.search":
		return object("items", "mode", "indexConflict")
	case "memory.entries.get":
		return object("memoryId", "scope", "content", "createdAt", "updatedAt")
	case "memory.entries.create", "memory.entries.update", "memory.entries.forget":
		return object("memoryId", "duplicate")
	case "interaction.questions.create", "interaction.questions.get":
		return object("questionId", "status", "message", "requestedSchema", "allowSkip", "content")
	}
	return nil
}
