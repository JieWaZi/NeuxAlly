package status_error

import (
	"testing"

	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/i18nx"
)

func TestChatHasActiveExecutionLocalizesInChinese(t *testing.T) {
	i18nx.Load(&conf.I18N{Langs: []string{"zh"}})
	message := ChatHasActiveExecution.Localize(i18nx.Instance(), "zh")
	if message.Key() != ChatHasActiveExecution.Key() ||
		message.Value() != "当前对话已有运行中的执行" {
		t.Fatalf(
			"localized message = key %q value %q, want key %q value %q",
			message.Key(),
			message.Value(),
			ChatHasActiveExecution.Key(),
			"当前对话已有运行中的执行",
		)
	}
}
