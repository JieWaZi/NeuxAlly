package aiaccount

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/constants"
	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/internal/accountconfig"
)

func TestQueuedPreparationSerializesSubscriptionRefreshAndCanBeCancelled(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "refresh-after-release"
		if cancelled {
			name = "cancel-while-waiting"
		}
		t.Run(name, func(t *testing.T) {
			service, account, _ := subscriptionFixture(t)
			ctx := context.Background()
			first := t.TempDir()
			_, release, err := service.Prepare(ctx, account.ID, constants.RuntimeIDCodex, first, "")
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, _, err := service.Prepare(ctx, account.ID, constants.RuntimeIDCodex, t.TempDir(), ""); !errors.Is(err, status_error.AIAccountBusy) {
				t.Fatalf("普通聊天准备不应改为后台等待: %v", err)
			}
			waitContext, cancel := context.WithCancel(ctx)
			defer cancel()
			second := t.TempDir()
			finished := make(chan error, 1)
			go func() {
				_, release, err := service.PrepareQueued(waitContext, account.ID, constants.RuntimeIDCodex, second, "")
				if release != nil {
					release()
				}
				finished <- err
			}()
			select {
			case err := <-finished:
				t.Fatalf("后台准备没有等待当前订阅调用: %v", err)
			case <-time.After(30 * time.Millisecond):
			}
			if cancelled {
				cancel()
			} else {
				// 第一调用轮换令牌后，第二调用才可投射最新凭据，不能复制同一旧令牌并行刷新。
				tokens := subscriptionTokens("user", "refresh-after-first-call")
				live := codexLiveAuth{LastRefresh: time.Now().UTC(), Tokens: codexLiveTokens{IDToken: tokens.IDToken, AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, AccountID: tokens.AccountID()}}
				data, _ := json.Marshal(live)
				if err := accountconfig.WriteFile(filepath.Join(accountconfig.Home(first, constants.RuntimeIDCodex), "auth.json"), data); err != nil {
					t.Fatal(err)
				}
				release()
			}
			select {
			case err := <-finished:
				if cancelled && !errors.Is(err, context.Canceled) || !cancelled && err != nil {
					t.Fatalf("后台准备结算错误: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("后台准备未响应释放或取消")
			}
			if cancelled {
				if _, err := service.Exclusive(account.ID); !errors.Is(err, status_error.AIAccountBusy) {
					t.Fatal("等待方取消释放了第一调用的租约")
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(accountconfig.Home(second, constants.RuntimeIDCodex), "auth.json"))
			if err != nil {
				t.Fatal(err)
			}
			var actual codexLiveAuth
			if err = json.Unmarshal(data, &actual); err != nil || actual.Tokens.RefreshToken != "refresh-after-first-call" {
				t.Fatalf("后台准备没有使用上次回收的刷新令牌: %v", err)
			}
		})
	}
}
