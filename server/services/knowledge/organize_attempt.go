package knowledge

import (
	"context"
	"sync"

	"github.com/JieWaZi/NeuxAlly/server/constants/status_error"
	"github.com/JieWaZi/NeuxAlly/server/repositories/models"
	"github.com/JieWaZi/neux-knowledge/core"
)

// organizeAttempt 只拥有当前尝试的取消上下文，不添加上游不存在的整批时间限制。
type organizeAttempt struct {
	attempt int                // 核心持久尝试号。
	ctx     context.Context    // 本次同步模型管线的上下文。
	cancel  context.CancelFunc // 服务关闭或尝试收束时释放真实模型调用。
	done    chan struct{}      // 标记当前驱动已经收束。
	finish  sync.Once          // 同一尝试只结束一次。
}

func (s *Service) attemptContext(job *core.Job, _ models.KnowledgeTaskConfiguration) (*organizeAttempt, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.closing || s.ctx == nil || s.ctx.Err() != nil {
		return nil, status_error.KnowledgeCancelled
	}
	if previous := s.attempts[job.ID]; previous != nil {
		if previous.attempt == job.Attempt {
			return previous, nil
		}
		if previous.attempt > job.Attempt {
			return nil, status_error.KnowledgeCancelled
		}
		previous.finish.Do(func() { close(previous.done); previous.cancel() })
	}
	ctx, cancel := context.WithCancel(s.ctx)
	attempt := &organizeAttempt{attempt: job.Attempt, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	if s.attempts == nil {
		s.attempts = map[string]*organizeAttempt{}
	}
	s.attempts[job.ID] = attempt
	return attempt, nil
}

func (s *Service) finishAttempt(job string, attempt *organizeAttempt) {
	attempt.finish.Do(func() { close(attempt.done); attempt.cancel() })
	s.mutex.Lock()
	if s.attempts[job] == attempt {
		delete(s.attempts, job)
	}
	s.mutex.Unlock()
}
