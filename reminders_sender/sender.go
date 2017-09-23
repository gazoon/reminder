package remsender

import (
	"context"
	"reminder/core"
	"reminder/core/presenter"
	"reminder/models"
	"reminder/pages"
	"reminder/storages/reminders"
	"sync"

	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/utils"
)

var (
	gLogger = logging.WithPackage("reminders_sender")
)

type Sender struct {
	*logging.ObjectLogger
	presenter  *presenter.UIPresenter
	source     reminders.Reader
	workersNum int
	wg         sync.WaitGroup
}

func NewSender(presenter *presenter.UIPresenter, source reminders.Reader, workersNum int) *Sender {
	logger := logging.NewObjectLogger("reminders_sender", nil)
	return &Sender{presenter: presenter, source: source, workersNum: workersNum, ObjectLogger: logger}
}

func (s *Sender) onReminder(ctx context.Context, reminder *models.Reminder) {
	s.GetLogger(ctx).Info("Reminder received: %s", reminder)
	showURL := core.NewURL("show_reminder", "when_ready", nil)
	req := &core.Request{Ctx: ctx, Msg: &pages.ReminderReadyMessage{reminder}, ChatID: reminder.ChatID, URL: showURL}
	s.presenter.HandleRequest(req)
}

func (s *Sender) Start() {
	gLogger.WithField("workers_num", s.workersNum).Info("Listening for incoming messages")
	for i := 0; i < s.workersNum; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				ctx := utils.PrepareContext(logging.NewRequestID())
				s.GetLogger(ctx).Info("Try to fetch new reminders")
				reminder, ok := s.source.GetNext(ctx)
				if !ok {
					return
				}
				s.onReminder(ctx, reminder)
			}
		}()
	}
}

func (s *Sender) Stop() {
	gLogger.Info("Close source for reading")
	s.source.StopGivingMsgs()
	gLogger.Info("Waiting until all workers will process the remaining reminders")
	s.wg.Wait()
	gLogger.Info("All workers've been stopped")
}
