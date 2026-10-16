// NeuxAlly Core 是伙伴工作台的本地业务与持久化进程。
package main

import (
	"context"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/JieWaZi/NeuxAlly/server/apis"
	"github.com/JieWaZi/NeuxAlly/server/apis/automatic_tasks"
	apiMiddleware "github.com/JieWaZi/NeuxAlly/server/apis/middleware"
	"github.com/JieWaZi/NeuxAlly/server/global"
	"github.com/JieWaZi/NeuxAlly/server/internal/logx"
	"github.com/JieWaZi/NeuxAlly/server/internal/parentwatch"
	workflowfunction "github.com/JieWaZi/NeuxAlly/server/internal/workflow/function"
	"github.com/shrewx/ginx"
	"github.com/spf13/cobra"
)

//go:generate toolx gen openapi

func main() {
	if workflowfunction.IsChildMode() {
		os.Exit(workflowfunction.RunChild(os.Stdin))
	}
	ginx.Launch(func(*cobra.Command, []string) {
		ginx.Parse(global.Config)
		ginx.SetCORSMiddleware(apiMiddleware.CORS())
		if err := global.Prepare(global.Config); err != nil {
			panic(err)
		}

		rootContext, cancel := context.WithCancel(context.Background())
		var parentWaitGroup sync.WaitGroup
		if parentPIDValue := os.Getenv("ALLY_DESKTOP_PARENT_PID"); parentPIDValue != "" {
			parentPID, err := strconv.Atoi(parentPIDValue)
			if err != nil || parentPID <= 0 {
				cancel()
				panic("ALLY_DESKTOP_PARENT_PID 必须是有效进程标识")
			}
			parentWaitGroup.Add(1)
			go func() {
				defer parentWaitGroup.Done()
				if !parentwatch.Wait(rootContext, parentPID, 100*time.Millisecond) {
					return
				}
				process, findErr := os.FindProcess(os.Getpid())
				if findErr == nil {
					_ = process.Signal(syscall.SIGTERM)
				}
			}()
		}
		if err := global.Load(rootContext, global.Config, automatic_tasks.NewLocalHandler); err != nil {
			cancel()
			parentWaitGroup.Wait()
			panic(err)
		}

		ginx.AddShutdownHook(func() {
			cancel()
			parentWaitGroup.Wait()
			if err := global.Close(); err != nil {
				logx.Errorf("close NeuxAlly Core failed: %v", err)
			}
		})
		ginx.RunServer(&global.Config.Server, apis.RootRouter)
	})
}
