package messageview

import (
	"encoding/json"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/agentcapability/contract"
	"github.com/JieWaZi/NeuxAlly/server/internal/chat/eventlog"
)

func (p *MessageViewBuilder) applyAgentCommand(record eventlog.Record) (string, error) {
	var command eventlog.AgentCommandData
	if json.Unmarshal(record.Data, &command) != nil || command.CommandID == "" || record.ExecutionID == "" {
		return "", status_error.ChatLogCorrupted
	}
	operation, exists := contract.Find(command.Operation)
	if !exists || command.Completion != operation.Completion {
		return "", status_error.ChatLogCorrupted
	}
	if ref := command.WorkflowRun; ref != nil && (ref.WorkflowID == "" || ref.RunID == "" || ref.RevisionID == "" || ref.Name == "") {
		return "", status_error.ChatLogCorrupted
	}
	if p.agentCommands[record.ExecutionID] == nil {
		p.agentCommands[record.ExecutionID] = map[string]eventlog.AgentCommandData{}
		p.agentMatches[record.ExecutionID] = map[string]string{}
	}
	if _, exists := p.agentCommands[record.ExecutionID][command.CommandID]; !exists {
		p.agentCommandOrder[record.ExecutionID] = append(p.agentCommandOrder[record.ExecutionID], command.CommandID)
	}
	p.agentCommands[record.ExecutionID][command.CommandID] = command
	messageID := p.executionMessages[record.ExecutionID]
	if index, exists := p.messageIndexes[messageID]; exists {
		message := &p.messages[index]
		if applyCommandReferences(message, command) {
			message.Revision = max(message.Revision+1, record.Sequence)
			message.UpdatedAt = record.RecordedAt
			return message.ID, nil
		}
	}
	return "", nil
}

// applyPendingCommandReferences 在助手消息晚于 Core 事实建立时补齐同一执行的引用。
func (p *MessageViewBuilder) applyPendingCommandReferences(message *Message) {
	for _, id := range p.agentCommandOrder[message.ExecutionID] {
		applyCommandReferences(message, p.agentCommands[message.ExecutionID][id])
	}
}

// applyCommandReferences 按真实运行及受管路径去重，命令重放不新增业务执行项或文件。
func applyCommandReferences(message *Message, command eventlog.AgentCommandData) bool {
	changed := false
	if ref := command.WorkflowRun; ref != nil {
		found := false
		for _, current := range message.WorkflowRuns {
			if current.RunID == ref.RunID {
				found = true
				break
			}
		}
		if !found {
			message.WorkflowRuns = append(message.WorkflowRuns, *ref)
			message.Retryable = false
			changed = true
		}
	}
	for _, attachment := range command.Attachments {
		if attachment.Path == "" {
			continue
		}
		found := false
		for _, current := range message.Attachments {
			if current.Path == attachment.Path {
				found = true
				break
			}
		}
		if !found {
			message.Attachments = append(message.Attachments, attachment)
			changed = true
		}
	}
	return changed
}

// matchAgentCommand 只改写实际工具行；找不到同执行 Core 事实时保持原生展示。
func (p *MessageViewBuilder) matchAgentCommand(executionID string, part *MessageDisplayPart, result map[string]any) {
	if part.ToolStatus == constants.MessageToolStatusFailed {
		return
	}
	response := commandReceipt(result, 0)
	if response == nil {
		return
	}
	id, _ := response["commandId"].(string)
	op, _ := response["operation"].(string)
	fact, exists := p.agentCommands[executionID][id]
	if !exists || fact.Operation != op || response["ok"] != true {
		return
	}
	if matched := p.agentMatches[executionID][id]; matched != "" && matched != part.ToolCallID {
		return
	}
	operation, exists := contract.Find(op)
	if !exists || operation.DisplayLabel() == "" {
		return
	}
	p.agentMatches[executionID][id] = part.ToolCallID
	part.ToolLabel, part.ToolLoadingLabel = completedAndLoadingPhrase(operation.DisplayLabel())
	part.ToolCategory, part.ToolAction, part.ToolSafeSummary = constants.MessageToolCategoryOther, constants.MessageToolActionOther, nil
}

// commandReceipt 只读取单个 JSON stdout 的回执身份，不把工具文本当成完成证据。
func commandReceipt(value any, depth int) map[string]any {
	if depth > 6 {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if len(typed) > constants.AgentCLIRequestMaximumBytes {
			return nil
		}
		var decoded any
		if json.Unmarshal([]byte(typed), &decoded) == nil {
			return commandReceipt(decoded, depth+1)
		}
	case map[string]any:
		if typed["protocolVersion"] == float64(constants.AgentCLIProtocolVersion) && typed["ok"] == true {
			return typed
		}
		for _, key := range []string{"stdout", "output", "text", "content", "result"} {
			if found := commandReceipt(typed[key], depth+1); found != nil {
				return found
			}
		}
	case []any:
		for _, item := range typed {
			if found := commandReceipt(item, depth+1); found != nil {
				return found
			}
		}
	}
	return nil
}
