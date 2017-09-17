package iterator

import (
	"reminder/core"

	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"strings"
)

var gLogger = logging.WithPackage("iterator")

type ctxKey int

const (
	SendTextCmd                  = "send_text"
	ClearPageStateCmd            = "clear_page_state"
	SendTextWithButtonsCmd       = "send_text_with_buttons"
	SendAttachmentCmd            = "send_attachment"
	SendAttachmentWithButtonsCmd = "send_attachment_with_buttons"
	SetInputHandlerCmd           = "set_input_handler"

	SendButtonsCmd = "send_buttons"
	ForeachCmd     = "foreach"
	pageNameCtxKey = ctxKey(1)
)

var foreachShortcuts = map[string]string{
	"send_texts":       SendTextCmd,
	"send_attachments": SendAttachmentCmd,
}

type Command struct {
	Name string
	Args interface{}
}

type Button struct {
	Text    string
	Handler *core.URL
	Intents []string
}

func (b Button) String() string {
	return logging.ObjToString(&b)
}

type Iterator struct {
	messenger  messenger.Messenger
	req        *core.Request
	initScript []*Command
	logger     *log.Entry
}

func NewCtxWithPageName(ctx context.Context, pageName string) context.Context {
	return context.WithValue(ctx, pageNameCtxKey, pageName)
}

func New(req *core.Request, script []*Command, messenger messenger.Messenger) *Iterator {
	logger := logging.FromContextAndBase(req.Ctx, gLogger)
	pageName := req.Ctx.Value(pageNameCtxKey)
	if pageName != nil {
		logger = logger.WithField("iteration_page", pageName)
	}
	return &Iterator{req: req, messenger: messenger, initScript: script, logger: logger}
}

func (iter *Iterator) sendText(args interface{}) error {
	text, err := processTextArgs(args)
	if err != nil {
		return err
	}
	_, err = iter.messenger.SendText(iter.req.Ctx, iter.req.ChatID, text)
	return errors.Wrap(err, "messenger send text")
}

func (iter *Iterator) clearPageState(args interface{}) error {
	pageName, ok := args.(string)
	if !ok {
		return errors.Errorf("called with not string arg %v", args)
	}
	delete(iter.req.Session.PagesStates, pageName)
	return nil
}

func (iter *Iterator) sendTextWithButtons(args interface{}) error {
	params, ok := args.(map[string]interface{})
	if !ok {
		return errors.Errorf("called with not json object arg %v", args)
	}
	text, err := processTextArgs(params["text"])
	if err != nil {
		return errors.Wrap(err, "'text' param")
	}
	buttons, err := getButtonsArg(params["buttons"])
	if err != nil {
		return errors.Wrap(err, "'buttons' param")
	}
	messengerButtons := make([]*messenger.Button, len(buttons))
	for i, button := range buttons {
		var payload string
		if button.Handler != nil {
			payload = button.Handler.Encode()
		} else {
			payload = button.Text
		}
		messengerButtons[i] = &messenger.Button{button.Text, payload}
		if button.Intents != nil {
			if button.Handler == nil {
				return errors.Errorf("button with intents without handler %+v", button)
			}
			iter.req.Session.AddIntent(button.Intents, button.Handler)
		}
	}
	iter.logger.WithFields(log.Fields{"text": text, "buttons": messengerButtons}).
		Info("Send text with connected buttons to the messenger")
	_, err = iter.messenger.SendTextWithButtons(iter.req.Ctx, iter.req.ChatID, text, messengerButtons...)
	return errors.Wrap(err, "messenger send text with buttons")
}

func (iter *Iterator) setInputHandler(args interface{}) error {
	handler, ok := args.(*core.URL)
	if !ok {
		return errors.Errorf("expected *core.URL arg, got %v", args)
	}
	iter.req.Session.SetInputHandler(iter.req.Ctx, handler)
	return nil
}

func (iter *Iterator) sendAttachment(args interface{}) error {
	return nil
}

func (iter *Iterator) sendAttachmentWithButtons(args interface{}) error {
	return nil
}

func dropEmptyCommands(originalScript []*Command) ([]*Command, error) {
	script := make([]*Command, 0, len(originalScript))
	for _, cmd := range originalScript {
		if cmd.Name == SendTextCmd {
			text, err := processTextArgs(cmd.Args)
			if err != nil {
				return nil, errors.Wrap(err, SendTextCmd)
			}
			if text == "" {
				continue
			}
		}
		if cmd.Name == SendButtonsCmd {
			buttons, err := getButtonsArg(cmd.Args)
			if err != nil {
				return nil, errors.Wrap(err, SendButtonsCmd)
			}
			if len(buttons) == 0 {
				continue
			}
		}
		script = append(script, cmd)
	}
	return script, nil
}

func unfoldForeach(originalScript []*Command) ([]*Command, error) {
	script := make([]*Command, 0, len(originalScript))
	for _, cmd := range originalScript {
		if originalFunc, ok := foreachShortcuts[cmd.Name]; ok {
			cmd = &Command{
				Name: ForeachCmd,
				Args: map[string]interface{}{"function": originalFunc, "values": cmd.Args},
			}
		}
		if cmd.Name != ForeachCmd {
			script = append(script, cmd)
			continue
		}
		foreach := &struct {
			Function string        `mapstructure:"function"`
			Values   []interface{} `mapstructure:"values"`
		}{}
		err := mapstructure.Decode(cmd.Args, foreach)
		if err != nil {
			return nil, errors.Wrapf(err, "bad args %v for foreach command", cmd.Args)
		}
		newCommands := make([]*Command, len(foreach.Values))
		for i, v := range foreach.Values {
			newCommands[i] = &Command{Name: foreach.Function, Args: v}
		}
		script = append(script, newCommands...)
	}
	return script, nil
}

func connectButtonsToMessage(script []*Command) ([]*Command, error) {
	originalScript := script
	script = make([]*Command, 0, len(originalScript))
	var lastMessageCmd *Command

	for _, cmd := range originalScript {
		switch cmd.Name {
		case SendTextCmd, SendAttachmentCmd:
			script = addNotNilCmd(script, lastMessageCmd)
			lastMessageCmd = cmd
			continue
		case SendTextWithButtonsCmd, SendAttachmentWithButtonsCmd:
			script = addNotNilCmd(script, lastMessageCmd)
			lastMessageCmd = nil
		case SendButtonsCmd:
			if lastMessageCmd == nil {
				return nil, errors.Errorf("there is no messages to connect buttons %v to", cmd.Args)
			}
			var withButtonsCmdName string
			withButtonsCmdArgs := map[string]interface{}{"buttons": cmd.Args}
			if lastMessageCmd.Name == SendTextCmd {
				withButtonsCmdName = SendTextWithButtonsCmd
				withButtonsCmdArgs["text"] = lastMessageCmd.Args
			} else {
				withButtonsCmdName = SendAttachmentWithButtonsCmd
				withButtonsCmdArgs["attachment"] = lastMessageCmd.Args
			}
			cmd = &Command{Name: withButtonsCmdName, Args: withButtonsCmdArgs}
			lastMessageCmd = nil
		}
		script = append(script, cmd)
	}
	script = addNotNilCmd(script, lastMessageCmd)
	return script, nil
}

func addNotNilCmd(script []*Command, cmd *Command) []*Command {
	if cmd != nil {
		script = append(script, cmd)
	}
	return script
}

func (iter *Iterator) Run() error {
	script := iter.initScript
	var err error
	script, err = dropEmptyCommands(script)
	if err != nil {
		return err
	}
	script, err = unfoldForeach(script)
	if err != nil {
		return err
	}
	script, err = connectButtonsToMessage(script)
	if err != nil {
		return err
	}
	return iter.execute(script)
}

func (iter *Iterator) execute(resultScript []*Command) error {
	commandsMapping := map[string]func(args interface{}) error{
		SendTextCmd:                  iter.sendText,
		SendTextWithButtonsCmd:       iter.sendTextWithButtons,
		SendAttachmentWithButtonsCmd: iter.sendAttachment,
		SendAttachmentCmd:            iter.sendAttachment,
		SetInputHandlerCmd:           iter.setInputHandler,
		ClearPageStateCmd:            iter.clearPageState,
	}
	for _, cmd := range resultScript {
		cmdHandler, ok := commandsMapping[cmd.Name]
		if !ok {
			return errors.Errorf("unknown command %s", cmd.Name)
		}
		iter.logger.WithFields(log.Fields{"command": cmd.Name, "args": cmd.Args}).Infof("execute command")
		err := cmdHandler(cmd.Args)
		if err != nil {
			return errors.Wrapf(err, "command %s failed", cmd.Name)
		}
	}
	return nil
}

func getButtonsArg(arg interface{}) ([]*Button, error) {
	buttons, ok := arg.([]*Button)
	if !ok {
		return nil, errors.Errorf("param must be []*Button, not %v", arg)
	}
	return buttons, nil
}

func processTextArgs(textArgs interface{}) (string, error) {
	lines, ok := textArgs.([]interface{})
	if !ok {
		lines = []interface{}{textArgs}
	}
	texts := make([]string, len(lines))
	for i, line := range lines {
		text, ok := line.(string)
		if !ok {
			return "", errors.Errorf("args must be a string or list of strings %v", line)
		}
		texts[i] = text
	}
	wholeText := strings.Join(texts, "\n")
	return wholeText, nil
}
