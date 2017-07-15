package pages

import (
	"bytes"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	templ "text/template"

	"reminder/core"
	"reminder/core/iterator"

	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

const (
	fileExtension    = ".yaml"
	pagesFolder      = "pages"
	evaluationMarker = "$"

	mainAction = "main"
)

var (
	fileContentParser func(data []byte, val interface{}) error = parseYAML
)

func parseYAML(data []byte, val interface{}) error {
	err := yaml.Unmarshal(data, val)
	return err
}

type MessageHandler func(req *core.Request) (string, error)

type Page interface {
	GetName() string
	GetIntents() []*core.Intent
	GetInputHandler(name string) (MessageHandler, bool)
	Enter(req *core.Request, action string, params map[string]interface{}) (string, error)
}

type SequenceItem struct {
	Key   string
	Value interface{}
}

type Controller func(req *core.Request, params map[string]interface{}) (map[string]interface{}, error)

type PageStructure struct {
	Intents []*core.Intent                      `yaml:"intents"`
	Actions map[string][]map[string]interface{} `yaml:"actions"`
	Config  map[string]interface{}              `yaml:"config"`
}

type BasePage struct {
	*logging.ObjectLogger
	Name              string
	messenger         messenger.Messenger
	globalController  Controller
	actionControllers map[string]Controller
	InputHandlers     map[string]MessageHandler

	ParsedPage  *PageStructure
	Intents     []*core.Intent
	actionViews map[string][]*SequenceItem
}

func newBasePage(name string, globalController Controller, actionControllers map[string]Controller,
	inputHandlers map[string]MessageHandler, messenger messenger.Messenger) (*BasePage, error) {

	filePath := path.Join(pagesFolder, name+fileExtension)
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "read page content")
	}
	parsedPage := new(PageStructure)
	err = fileContentParser(fileContent, parsedPage)
	if err != nil {
		return nil, errors.Wrapf(err, "content parsing failed, file=%s", filePath)
	}
	actions, err := retrieveActions(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve actions")
	}
	if _, ok := actions[mainAction]; !ok {
		return nil, errors.Errorf("actions doesn't contain the main %v", actions)
	}
	logger := logging.NewObjectLogger("pages", log.Fields{"page": name})

	page := &BasePage{
		Name: name, messenger: messenger, globalController: globalController, actionControllers: actionControllers,
		InputHandlers: inputHandlers, ParsedPage: parsedPage, Intents: parsedPage.Intents, actionViews: actions,
		ObjectLogger: logger,
	}
	return page, nil
}

func retrieveActions(parsedPage *PageStructure) (map[string][]*SequenceItem, error) {
	actions := make(map[string][]*SequenceItem, len(parsedPage.Actions))
	for actionName, actionData := range parsedPage.Actions {
		seq, err := iterationPartToSequence(actionData)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot transform action %s to sequence", actionName)
		}
		actions[actionName] = seq
	}
	return actions, nil
}

func iterationPartToSequence(iterationPart []map[string]interface{}) ([]*SequenceItem, error) {
	sequence := make([]*SequenceItem, len(iterationPart))
	for idx, data := range iterationPart {
		if len(data) != 1 {
			return nil, errors.Errorf("sequence item must have one key-value pair %s", data)
		}
		item := &SequenceItem{}
		for key, value := range data {
			// there is only one cycle
			item.Key = key
			item.Value = value
		}
		sequence[idx] = item
	}
	return sequence, nil
}

func (bp *BasePage) Enter(req *core.Request, action string, params map[string]interface{}) (string, error) {
	var globalData map[string]interface{}
	if bp.globalController != nil {
		var err error
		globalData, err = bp.globalController(req, params)
		if err != nil {
			return "", errors.Wrap(err, "page global controller failed")
		}
	}
	if action == "" {
		action = mainAction
	}
	var actionData map[string]interface{}
	if controller, ok := bp.actionControllers[action]; ok {
		var err error
		actionData, err = controller(req, params)
		if err != nil {
			return "", errors.Wrapf(err, "controller %s failed", action)
		}
	}
	commonData := bp.getCommonScriptData(req, params)
	scriptData := mergeScriptData(actionData, globalData, commonData)
	redirectURI, err := bp.renderResponse(req, action, scriptData)
	return redirectURI, errors.Wrap(err, "response failed")
}

func (bp *BasePage) GetName() string {
	return bp.Name
}

func (bp *BasePage) GetIntents() []*core.Intent {
	return bp.Intents
}

func (bp *BasePage) GetInputHandler(name string) (MessageHandler, bool) {
	handler, ok := bp.InputHandlers[name]
	return handler, ok
}

func (bp *BasePage) actionNames() []string {
	names := make([]string, 0, len(bp.actionViews))
	for k := range bp.actionViews {
		names = append(names, k)
	}
	return names
}

func (bp *BasePage) getState(req *core.Request) map[string]interface{} {
	return req.Session.PagesStates[bp.Name]
}

func (bp *BasePage) updateState(req *core.Request, key string, value interface{}) {
	state, ok := req.Session.PagesStates[bp.Name]
	if !ok {
		state = make(map[string]interface{})
	}
	state[key] = value
}

func (bp *BasePage) clearState(req *core.Request) {
	delete(req.Session.PagesStates, bp.Name)
}

func (bp *BasePage) setState(req *core.Request, state map[string]interface{}) {
	req.Session.PagesStates[bp.Name] = state
}

func (bp *BasePage) getCommonScriptData(req *core.Request, params map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"message_text": req.Msg.Text,
		"user":         req.User,
		"chat":         req.Chat,
		"params":       params,
	}
	return data
}

func (bp *BasePage) renderResponse(req *core.Request, actionName string, data map[string]interface{}) (string, error) {
	nextAction, ok := bp.actionViews[actionName]
	if !ok {
		return "", errors.Errorf("action %s not found in %v", actionName, bp.actionNames())
	}
	visitedActions := map[string]bool{actionName: true}
	var script []*iterator.Command
	var redirectURI string
	for nextAction != nil {
		currentAction := nextAction
		nextAction = nil
		for _, item := range currentAction {
			cmd := &iterator.Command{Name: item.Key}
			evaluated, err := evaluateArgs(item.Value, data)
			if err != nil {
				return "", errors.Wrapf(err, "args evaluation failed, args=%v data=%v command=%s", item.Value, data, cmd.Name)
			}
			cmd.Args, err = computeConditionalStmts(evaluated)
			if err != nil {
				return "", errors.Wrapf(err, "cannot compute cond statements, args=%v command=%s", evaluated, cmd.Name)
			}
			if cmd.Name == "goto" {
				if cmd.Args == nil {
					continue
				}
				actionName, ok := cmd.Args.(string)
				if !ok {
					return "", errors.Errorf("goto argument must be a string, not %v", cmd.Args)
				}
				if isVisited := visitedActions[actionName]; isVisited {
					return "", errors.Errorf("actions cycle, already visited action %s", actionName)
				}
				visitedActions[actionName] = true
				nextAction, ok = bp.actionViews[actionName]
				if !ok {
					return "", errors.Errorf("goto to unexisting page action %s, actions=%v", actionName, bp.actionNames())
				}
				break
			} else if cmd.Name == "redirect" {
				if cmd.Args == nil {
					continue
				}
				var ok bool
				redirectURI, ok = cmd.Args.(string)
				if !ok {
					return "", errors.Errorf("redirect argument must be a string, not %v", cmd.Args)
				}
				break
			} else {
				script = append(script, cmd)
			}
		}
	}
	err := bp.executeScript(req, script)
	if err != nil {
		return "", err
	}

	return redirectURI, nil
}

func (bp *BasePage) executeScript(req *core.Request, script []*iterator.Command) error {
	if len(script) == 0 {
		return nil
	}
	req.Ctx = iterator.NewCtxWithPageName(req.Ctx, bp.Name)
	iter := iterator.New(req, script, bp.messenger)
	err := iter.Run()
	return errors.Wrap(err, "script iteration falied")
}

func evaluateArgs(args interface{}, scriptData map[string]interface{}) (interface{}, error) {
	var evaluatedValue interface{}
	if textArg, ok := args.(string); ok {
		if strings.HasPrefix(textArg, evaluationMarker) {
			dataKey := strings.TrimLeft(textArg, evaluationMarker)
			var err error
			evaluatedValue, err = retrieveValue(dataKey, scriptData)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot retrieve value for %s, from %v", dataKey, scriptData)
			}
		} else {
			b := bytes.Buffer{}
			err := templ.Must(templ.New("arg").Parse(textArg)).Execute(&b, scriptData)
			if err != nil {
				return nil, errors.Wrap(err, "template execute failed")
			}
			evaluatedValue = b.String()
		}
	} else if arrayArg, ok := args.([]interface{}); ok {
		evaluatedArray := make([]interface{}, len(arrayArg))
		for i, args := range arrayArg {
			var err error
			evaluatedArray[i], err = evaluateArgs(args, scriptData)
			if err != nil {
				return nil, err
			}
		}
		evaluatedValue = evaluatedArray
	} else if objectArg, ok := args.(map[string]interface{}); ok {
		evaluatedObject := make(map[string]interface{}, len(objectArg))
		for key, args := range objectArg {
			var err error
			evaluatedObject[key], err = evaluateArgs(args, scriptData)
			if err != nil {
				return nil, err
			}
		}
		evaluatedValue = evaluatedObject
	} else {
		evaluatedValue = args
	}
	return evaluatedValue, nil
}

func computeConditionalStmts(args interface{}) (interface{}, error) {
	if arrayArg, ok := args.([]interface{}); ok {
		computedArray := make([]interface{}, len(arrayArg))
		for i, args := range arrayArg {
			var err error
			computedArray[i], err = computeConditionalStmts(args)
			if err != nil {
				return nil, err
			}
		}
		return computedArray, nil
	}
	if objectArg, ok := args.(map[string]interface{}); ok {
		condArg, ok := objectArg["cond"]
		var value interface{}
		if ok {
			if len(objectArg) > 1 {
				return nil, errors.Errorf("the cond must be the only one field in the object %v", objectArg)
			}
			var err error
			value, err = condStatement(condArg)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid cond stmt %v", condArg)
			}
		} else {
			var err error
			value, err = ifStatement(objectArg)
			if err != nil {
				computedObject := make(map[string]interface{}, len(objectArg))
				for key, args := range objectArg {
					var err error
					computedObject[key], err = computeConditionalStmts(args)
					if err != nil {
						return nil, err
					}
				}
				return computedObject, nil
			}
		}
		return computeConditionalStmts(value)
	}
	return args, nil

}

func ifStatement(item map[string]interface{}) (interface{}, error) {
	ifArg := item["if"]
	if ifArg == nil {
		return nil, errors.New("if key doesn't present")
	}
	condition, ok := ifArg.(bool)
	if !ok {
		var err error
		stringArg, _ := ifArg.(string)
		condition, err = strconv.ParseBool(stringArg)
		if err != nil {
			return nil, errors.Wrap(err, "if arg must be bool or a string with bool parsable value")
		}
	}
	if condition {
		return item["then"], nil
	} else {
		return item["else"], nil
	}
}

func condStatement(data interface{}) (interface{}, error) {
	ifsArray, ok := data.([]interface{})
	if !ok {
		return nil, errors.Errorf("cond arg must be an array: %v", data)
	}
	for _, item := range ifsArray {
		ifData, ok := item.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("cond arg must be array of objects, found %v", item)
		}
		value, err := ifStatement(ifData)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid if stmt %v", ifData)
		}
		if value != nil {
			return value, nil
		}
	}
	return nil, nil
}

func retrieveValue(dataKey string, scriptData interface{}) (interface{}, error) {
	lookupFields := strings.Split(strings.ToLower(dataKey), ".")
	var value interface{} = scriptData
	for _, field := range lookupFields {
		if index, err := strconv.Atoi(field); err == nil {
			array, ok := value.([]interface{})
			if !ok {
				return nil, errors.Errorf("%v not an array, lookup index=%d", value, index)
			}
			if index < 0 || index >= len(array) {
				return nil, errors.Errorf("index %d out of range %v", index, value)
			}
			value = array[index]

		} else if obj, ok := value.(map[string]interface{}); ok {
			key := field

			if !ok {
				return nil, errors.Errorf("%v not a json object, lookup key=%s", value, key)
			}
			value, ok = obj[key]
			if !ok {
				return nil, errors.Errorf("key %s not found in %v", key, obj)
			}
		} else if structValue, ok := toStructValue(value); ok {
			field = strings.Title(field)
			fieldValue := structValue.FieldByName(field)
			if !fieldValue.IsValid() {
				return nil, errors.Errorf("field %s not present in %+v", field, value)
			}
			if !fieldValue.CanInterface() {
				return nil, errors.Errorf("unexported field %s %v", field, value)
			}
			value = fieldValue.Interface()
		} else {
			return nil, errors.Errorf("%v is not a json object and not a struct, lookup key=%s", value, field)
		}
	}
	return value, nil
}

func toStructValue(value interface{}) (reflect.Value, bool) {
	v := reflect.Indirect(reflect.ValueOf(value))
	return v, v.Kind() == reflect.Struct
}

func mergeScriptData(actionData, globalPageData, commonData map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(actionData)+len(globalPageData)+len(commonData))
	//		priority common<global<action
	for k, v := range commonData {
		result[k] = v
	}
	for k, v := range globalPageData {
		result[k] = v
	}
	for k, v := range actionData {
		result[k] = v
	}
	return result
}

func NewHomePage(messenger messenger.Messenger) (*BasePage, error) {
	return newBasePage("home", nil, nil, nil, messenger)
}

type ReminderListPage struct {
	*BasePage
	PreviewTemplate string
}

func (rl *ReminderListPage) getOrDeleteInputHandler(req *core.Request) (string, error) {
	return "", nil
}

func (rl *ReminderListPage) mainAction(req *core.Request, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"has_reminders": true,
		"foo":           map[string]interface{}{"bar": []interface{}{2, 3, "4"}},
	}
	return data, nil
}

func (rl *ReminderListPage) fooController(req *core.Request, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"foo": "foo",
	}
	return data, nil
}

func (rl *ReminderListPage) barController(req *core.Request, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"foo": "bar",
	}
	return data, nil
}

func (rl *ReminderListPage) globalController(req *core.Request, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"reminder_previews": []interface{}{"foo", "bbbbbb", "222"},
	}
	return data, nil
}

func NewReminderListPage(messenger messenger.Messenger) (*ReminderListPage, error) {
	page := new(ReminderListPage)
	inputs := map[string]MessageHandler{
		"on_get_or_delete": page.getOrDeleteInputHandler,
	}
	controllers := map[string]Controller{
		mainAction:      page.mainAction,
		"has_reminders": page.fooController,
		"no_reminders":  page.barController,
	}
	basePage, err := newBasePage("reminder_list", page.globalController, controllers, inputs, messenger)
	if err != nil {
		return nil, err
	}
	previewTemplate, _ := basePage.ParsedPage.Config["preview_template"].(string)
	if len(previewTemplate) == 0 {
		return nil, errors.Errorf("config doesn't contain preview template %v", basePage.ParsedPage.Config)
	}
	return &ReminderListPage{BasePage: basePage, PreviewTemplate: previewTemplate}, nil
}
