package channel

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	channeltool "github.com/JieWaZi/NeuxAlly/server/internal/teamcapability"
	"github.com/JieWaZi/NeuxAlly/server/repositories/controllers"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	chatservice "github.com/JieWaZi/NeuxAlly/server/services/chat"
	"github.com/JieWaZi/NeuxAlly/server/services/execution"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
)

// leaderTurnChatGroupFake 提供首发路由测试使用的成员 Chat，不启动真实 Runtime。
type leaderTurnChatGroupFake struct {
	// chats 保存成员 Chat 摘要，供负责人繁忙检查和重启恢复读取。
	chats *postTurnChatFake
}

// PrepareChatGroup 为每位成员返回独立 Chat，并模拟首发执行期间负责人繁忙。
func (f *leaderTurnChatGroupFake) PrepareChatGroup(
	_ context.Context,
	intent execution.PrepareChatGroupIntent,
) (execution.PreparedChatGroup, error) {
	f.chats.mutex.Lock()
	defer f.chats.mutex.Unlock()
	result := execution.PreparedChatGroup{}
	for _, member := range intent.Members {
		chat := models.Chat{
			ID: member.ChatID, WakerID: member.WakerID, Title: intent.Title,
			WorkspaceDirectory: intent.Scope.WorkingDirectory,
		}
		result.Chats = append(result.Chats, chat)
		if member.WakerID == intent.Scope.WakerID {
			chat.ActiveExecutionID = "initial-execution"
		}
		f.chats.summaries[chat.ID] = chatservice.ChatSummary{Chat: chat}
	}
	return result, nil
}

// TestLeaderTurnModeSurvivesFirstSendAndQueueRecovery 验证首发与重启恢复均向执行层传递本轮意图。
func TestLeaderTurnModeSurvivesFirstSendAndQueueRecovery(t *testing.T) {
	for _, initialDirect := range []bool{false, true} {
		t.Run(fmt.Sprintf("initial_direct=%t", initialDirect), func(t *testing.T) {
			database, err := dbhelper.NewDB(conf.DB{
				Type: conf.Sqlite, DBName: filepath.Join(t.TempDir(), "leader-turn.db"), Migrate: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := database.DB.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			ctx := context.Background()
			channels := controllers.NewChannelController(database.DB)
			threads := controllers.NewChannelThreadController(database.DB)
			tasks := controllers.NewChannelTaskController(database.DB)
			wakers := controllers.NewWakerController(database.DB)
			leadID := "11111111111111111111111111111111"
			memberID := "22222222222222222222222222222222"
			channelID := "33333333333333333333333333333333"
			createChannelTestWakers(t, ctx, wakers, leadID, memberID)
			if err := channels.Create(ctx, models.Channel{
				ID: channelID, Name: "回合路由团队", LeadWakerID: leadID,
			}, []models.ChannelMember{
				{ChannelID: channelID, WakerID: leadID, RuntimeID: "codex", AccountID: "lead-account", PermissionLevel: constants.PermissionLevelAuto},
				{ChannelID: channelID, WakerID: memberID, RuntimeID: "codex", PermissionLevel: constants.PermissionLevelAuto, Position: 1},
			}); err != nil {
				t.Fatal(err)
			}
			chats := &postTurnChatFake{summaries: make(map[string]chatservice.ChatSummary)}
			executions := &postTurnExecutionFake{}
			newService := func() *service {
				return &service{
					channels: channels, threads: threads, tasks: tasks, wakers: wakers,
					transactions: controllers.NewTransactionController(dbhelper.NewGormTransactionManager(database.DB)),
					chats:        chats, executions: executions, chatGroups: &leaderTurnChatGroupFake{chats: chats},
					watchers: make(map[string]struct{}), capabilities: channeltool.NewRegistry(),
				}
			}
			sut := newService()
			if err := sut.Start(ctx); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(sut.Close)
			mention := ""
			if initialDirect {
				mention = leadID
			}
			input := execution.WorkInput{Actor: "user", Content: "修改项目文件并验证"}
			created, err := sut.createThread(ctx, CreateThreadInput{
				ChannelID: channelID, ClientRequestID: "first-send", WorkingDirectory: t.TempDir(),
				MentionWakerID: mention, Input: input,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertLeaderTurnIntent(t, executions.firstStart(), input.Content, initialDirect)
			if created.Members[0].AccountID != "lead-account" {
				t.Fatalf("account was not frozen in thread: %+v", created.Members)
			}
			if len(created.Tasks) != 0 {
				t.Fatalf("lead input created tasks: %+v", created.Tasks)
			}

			// 下一条使用相反模式且正文完全相同，必须只依赖结构化提及，不受历史回合影响。
			mention = leadID
			if initialDirect {
				mention = ""
			}
			queued, err := sut.sendMessage(ctx, SendMessageInput{
				ThreadID: created.Thread.ID, MentionWakerID: mention, Input: input,
			})
			if err != nil {
				t.Fatal(err)
			}
			if queued.QueuedInput == nil || queued.Execution != nil || queued.Task != nil || executions.startCount() != 1 {
				t.Fatalf("busy lead input was not queued: %+v", queued)
			}
			sut.Close()
			chats.mutex.Lock()
			summary := chats.summaries[created.Thread.LeadChatID]
			summary.Chat.ActiveExecutionID = ""
			chats.summaries[created.Thread.LeadChatID] = summary
			chats.mutex.Unlock()
			recovered := newService()
			if err := recovered.Start(ctx); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(recovered.Close)
			if executions.startCount() != 2 {
				t.Fatalf("recovered executions = %d, want 2", executions.startCount())
			}
			assertLeaderTurnIntent(t, executions.startAt(1), input.Content, !initialDirect)
		})
	}
}

// assertLeaderTurnIntent 检查执行边界保留原文和权限，并只包含当前回合的路由意图。
func assertLeaderTurnIntent(t *testing.T, intent execution.WorkIntent, content string, direct bool) {
	t.Helper()
	if intent.Input.Content != content || intent.Options.AccountID != "lead-account" || intent.Options.PermissionLevel != constants.PermissionLevelAuto || intent.Options.WorkspaceReadOnly {
		t.Fatalf("lead input or permission changed: %+v", intent)
	}
	instructions := intent.Options.SupplementalInstructions
	if len(instructions) != 1 || instructions[0].Name != constants.TeamInstructionFileName {
		t.Fatalf("lead instructions = %+v", instructions)
	}
	want, unwanted := "This is a coordination turn", "This is a direct-execution turn"
	if direct {
		want, unwanted = unwanted, want
	}
	if !strings.Contains(instructions[0].Content, want) || strings.Contains(instructions[0].Content, unwanted) {
		t.Fatalf("direct=%t instructions did not preserve turn mode: %s", direct, instructions[0].Content)
	}
}
