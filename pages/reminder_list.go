package pages

import (
	"reminder/core"
	"reminder/core/page"

	"fmt"
	"reminder/models"
	"reminder/storages/reminders"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type ReminderList struct {
	*page.BasePage

	Reminders reminders.Storage
}

func (rl *ReminderList) Init(builder *page.PagesBuilder) error {
	controllers := map[string]page.Controller{
		"on_get_or_delete": rl.getOrDeleteInputController,
	}
	var err error
	rl.BasePage, err = builder.NewBasePage("reminder_list", rl.globalController, controllers)
	return err
}

func (rl *ReminderList) getReminders(req *core.Request) ([]*models.Reminder, error) {
	list, err := rl.Reminders.List(req.Ctx, req.Chat.ID)
	return list, errors.Wrap(err, "storage list")
}

func parseGetOrDelete(req *core.Request) (string, int, error) {
	parts := strings.Split(req.Msg.Text, " ")
	if len(parts) != 2 {
		return "", 0, errors.New("expected 2 words input")
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, errors.New("the second word must be a number")
	}
	if index < 1 {
		return "", 0, errors.New("number must start from 1")
	}
	index--
	command := strings.ToLower(parts[0])
	if command != "show" && command != "delete" {
		return "", 0, errors.New("the first word must 'show' or 'delete'")
	}
	return command, index, nil
}

func (rl *ReminderList) getOrDeleteInputController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	command, index, err := parseGetOrDelete(req)
	if err != nil {
		return page.BadInputResponse(err.Error())
	}
	remindersList, err := rl.getReminders(req)
	if err != nil {
		return nil, nil, err
	}
	if index >= len(remindersList) {
		return page.BadInputResponse("there is no reminder with such number")
	}
	reminder := remindersList[index]
	var isDeleted bool
	if command == "delete" {
		err := rl.Reminders.Delete(req.Ctx, reminder.ID)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot delete from storage")
		}
		isDeleted = true
	}

	return map[string]interface{}{"error": false, "deleted": isDeleted, "reminder_id": reminder.ID}, nil, nil
}

func (rl *ReminderList) globalController(req *core.Request) (map[string]interface{}, *core.URL, error) {
	chatReminders, err := rl.getReminders(req)
	if err != nil {
		return nil, nil, err
	}
	previews := make([]interface{}, len(chatReminders))
	for i, reminder := range chatReminders {
		previews[i] = fmt.Sprintf("%d. %.30s", i+1, reminder.Title)
	}
	data := map[string]interface{}{
		"no_reminders":      len(chatReminders) == 0,
		"reminder_previews": previews,
	}
	return data, nil, nil
}
