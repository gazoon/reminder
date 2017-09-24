package page

import (
	"bytes"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	templ "text/template"

	"reminder/core"

	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/gazoon/bot_libs/logging"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/ghodss/yaml"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

const (
	yamlFileExtension  = ".yaml"
	DefaultPagesFolder = "pages"
	evaluationMarker   = "$"
	redirectCmd        = "redirect"
	gotoCmd            = "goto"
)

func parseYAML(data []byte, val interface{}) error {
	err := yaml.Unmarshal(data, val)
	return err
}

type Page interface {
	GetName() string
	Init(builder *PagesBuilder) error
	HandleIntent(req *core.Request) (*core.URL, error)
	Enter(req *core.Request) (*core.URL, error)
}

type SequenceItem struct {
	Key   string
	Value interface{}
}

type Controller func(req *core.Request) (map[string]interface{}, *core.URL, error)

type PageStructure struct {
	Intents []*struct {
		HandlerURLStr string   `json:"handler"`
		Words         []string `json:"words"`
	} `json:"intents"`
	Actions     map[string][]map[string]interface{} `json:"actions"`
	Config      map[string]interface{}              `json:"config"`
	EntryAction string                              `json:"entry_action"`
}

type PagesBuilder struct {
	messenger         messenger.Messenger
	fileExtension     string
	pagesFolder       string
	fileContentParser func(data []byte, val interface{}) error
}

func NewPagesBuilder(messenger messenger.Messenger, folder string) *PagesBuilder {
	return &PagesBuilder{messenger: messenger, fileExtension: yamlFileExtension, pagesFolder: folder,
		fileContentParser: parseYAML}
}

func (pb *PagesBuilder) NewBasePage(name string, globalController Controller, actionControllers map[string]Controller) (*BasePage, error) {
	parsedPage := new(PageStructure)
	var actionViews map[string][]*SequenceItem
	filePath := path.Join(pb.pagesFolder, name+pb.fileExtension)
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "read page content")
	}
	err = pb.fileContentParser(fileContent, parsedPage)
	if err != nil {
		return nil, errors.Wrapf(err, "content parsing failed, file=%s", filePath)
	}
	actionViews, err = retrieveActions(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve actions")
	}
	if _, viewExists := actionViews[parsedPage.EntryAction]; !viewExists {
		if _, controllerExists := actionControllers[parsedPage.EntryAction]; !controllerExists {
			return nil, errors.Errorf("entry action %s not found in page views or controllers", parsedPage.EntryAction)
		}
	}
	logger := logging.NewObjectLogger("pages", log.Fields{"page": name})

	page := &BasePage{
		Name: name, messenger: pb.messenger, globalController: globalController, actionControllers: actionControllers,
		ParsedPage: parsedPage, actionViews: actionViews, ObjectLogger: logger, entryAction: parsedPage.EntryAction,
	}

	page.Intents, err = page.buildIntents()
	if err != nil {
		return nil, errors.Wrap(err, "cannot build intents")
	}
	return page, nil
}

func (pb *PagesBuilder) InstantiatePages(pages ...Page) (map[string]Page, error) {
	registry := make(map[string]Page, len(pages))
	for _, p := range pages {
		err := p.Init(pb)
		if err != nil {
			return nil, errors.Wrapf(err, "page %s initialization failed", reflect.TypeOf(p))
		}
		registry[p.GetName()] = p
	}
	return registry, nil
}

type BasePage struct {
	*logging.ObjectLogger
	Name              string
	messenger         messenger.Messenger
	globalController  Controller
	actionControllers map[string]Controller

	ParsedPage  *PageStructure
	Intents     []*core.Intent
	actionViews map[string][]*SequenceItem
	entryAction string
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

func (bp *BasePage) Enter(req *core.Request) (*core.URL, error) {
	var globalData map[string]interface{}
	if bp.globalController != nil {
		var err error
		var redirectURI *core.URL
		globalData, redirectURI, err = bp.globalController(req)
		if err != nil {
			return nil, errors.Wrap(err, "page global controller failed")
		}
		if redirectURI != nil {
			bp.GetLogger(req.Ctx).Info("Global controller returns uri: %s", redirectURI.Encode())
			return redirectURI, nil
		}
	}
	var actionData map[string]interface{}
	action := bp.GetRequestAction(req)
	if controller, ok := bp.actionControllers[action]; ok {
		var err error
		var redirectURI *core.URL
		actionData, redirectURI, err = controller(req)
		if err != nil {
			return nil, errors.Wrapf(err, "controller %s failed", action)
		}
		if redirectURI != nil {
			bp.GetLogger(req.Ctx).Info("Action controller returns uri: %s", redirectURI.Encode())
			return redirectURI, nil
		}
	}
	commonData := bp.getCommonScriptData(req)
	scriptData := mergeScriptData(actionData, globalData, commonData)
	redirectURI, err := bp.renderResponse(req, scriptData)
	return redirectURI, errors.Wrap(err, "response failed")
}

func (bp *BasePage) GetName() string {
	return bp.Name
}

func (bp *BasePage) HandleIntent(req *core.Request) (*core.URL, error) {
	bp.GetLogger(req.Ctx).Info(req.Intents)
	return core.NotFoundPageURL, nil
}

func (bp *BasePage) ActionViews() []string {
	names := make([]string, 0, len(bp.actionViews))
	for k := range bp.actionViews {
		names = append(names, k)
	}
	return names
}

func (bp *BasePage) toAbsoluteURL(u *core.URL) *core.URL {
	result := u.Copy()
	if !result.IsRelative() {
		return result
	}
	result.Page = bp.Name
	return result
}

func (bp *BasePage) buildURL(action string, params map[string]string) *core.URL {
	return core.NewURL(bp.Name, action, params)
}

func (bp *BasePage) buildIntents() ([]*core.Intent, error) {
	parsedPage := bp.ParsedPage
	intents := make([]*core.Intent, len(parsedPage.Intents))
	for i, item := range parsedPage.Intents {
		intent, err := core.NewIntentStrHandler(item.HandlerURLStr, item.Words)
		if err != nil {
			return nil, err
		}
		intent.Handler = bp.toAbsoluteURL(intent.Handler)
		intents[i] = intent
	}
	return intents, nil
}

// global session state and page state always should be not nil maps
func (bp *BasePage) GetState(req *core.Request) map[string]interface{} {
	state, ok := req.Session.PagesStates[bp.Name]
	if !ok {
		state = make(map[string]interface{})
	}
	return state
}

func (bp *BasePage) UpdateState(req *core.Request, key string, value interface{}) {
	state := bp.GetState(req)
	state[key] = value
	bp.SetState(req, state)
}

func (bp *BasePage) ClearState(req *core.Request) {
	delete(req.Session.PagesStates, bp.Name)
}

func (bp *BasePage) SetState(req *core.Request, state map[string]interface{}) {
	req.Session.PagesStates[bp.Name] = state
}

func (bp *BasePage) getCommonScriptData(req *core.Request) map[string]interface{} {
	params := make(map[string]interface{}, len(req.URL.Params))
	for k, v := range req.URL.Params {
		params[k] = v
	}
	data := map[string]interface{}{
		"message_text": req.MsgText,
		"session":      req.Session.GlobalState,
		"page_state":   req.Session.PagesStates[bp.Name],
		"params":       params,
	}
	return data
}

func (bp *BasePage) renderResponse(req *core.Request, data map[string]interface{}) (*core.URL, error) {
	actionName := bp.GetRequestAction(req)
	nextAction, ok := bp.actionViews[actionName]
	if !ok {
		return nil, errors.Errorf("there is no action view for %s", actionName)
	}
	visitedActions := map[string]bool{actionName: true}
	var script []*Command
	var redirectURI *core.URL
	for nextAction != nil {
		currentAction := nextAction
		nextAction = nil
		for _, item := range currentAction {
			cmd := &Command{Name: item.Key}
			evaluated, err := evaluateArgs(item.Value, data)
			if err != nil {
				return nil, errors.Wrapf(err, "args evaluation failed, args=%v data=%v command=%s", item.Value, data, cmd.Name)
			}
			computed, err := computeConditionalStmts(evaluated)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot compute cond statements, args=%v command=%s", evaluated, cmd.Name)
			}
			transformed, err := bp.transformURLs(cmd.Name, computed)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot parse string urls to objects, args=%v command=%s", computed, cmd.Name)
			}
			cmd.Args = transformed
			if cmd.Name == gotoCmd {
				if cmd.Args == nil {
					continue
				}
				gotoAction, ok := cmd.Args.(string)
				if !ok {
					return nil, errors.Errorf("goto argument must be a string, not %v", cmd.Args)
				}
				if isVisited := visitedActions[gotoAction]; isVisited {
					return nil, errors.Errorf("actions cycle, already visited action %s", gotoAction)
				}
				visitedActions[gotoAction] = true
				nextAction, ok = bp.actionViews[gotoAction]
				if !ok {
					return nil, errors.Errorf("goto to nonexistent page action %s, actions=%v", gotoAction, bp.ActionViews())
				}
				break
			} else if cmd.Name == redirectCmd {
				if cmd.Args == nil {
					continue
				}

				redirectURI, ok = cmd.Args.(*core.URL)
				if !ok {
					return nil, errors.Errorf("redirect argument must be *core.URL, not %v", cmd.Args)
				}
				break
			} else {
				script = append(script, cmd)
			}
		}
	}
	err := bp.executeScript(req, script)
	if err != nil {
		return nil, err
	}

	return redirectURI, nil
}

func (bp *BasePage) parseButtonsArg(buttonsData interface{}) ([]*Button, error) {
	buttonsArray, ok := buttonsData.([]interface{})
	if !ok {
		return nil, errors.Errorf("expected array, not %v", buttonsData)
	}
	buttons := make([]*Button, len(buttonsArray))
	for i, buttonData := range buttonsArray {
		parsedButton := &struct {
			Text    string   `mapstructure:"text"`
			Handler string   `mapstructure:"handler"`
			Intents []string `mapstructure:"intents"`
		}{}
		err := mapstructure.Decode(buttonData, parsedButton)
		if err != nil {
			return nil, err
		}
		var handlerURL *core.URL
		if parsedButton.Handler != "" {
			handlerURL, err = bp.parseURL(parsedButton.Handler)
			if err != nil {
				return nil, errors.Wrapf(err, "incorrect button handler url %s", parsedButton.Handler)
			}
		}
		buttons[i] = &Button{Text: parsedButton.Text, Intents: parsedButton.Intents, Handler: handlerURL}
	}
	return buttons, nil
}

func (bp *BasePage) transformURLs(commandName string, args interface{}) (interface{}, error) {
	if commandName == redirectCmd || commandName == SetInputHandlerCmd {
		if args == nil {
			return args, nil
		}
		urlStr, ok := args.(string)
		if !ok {
			return nil, errors.Errorf("%s argument must be a string, not %v", commandName, args)
		}
		u, err := bp.parseURL(urlStr)
		if err != nil {
			return nil, errors.Wrapf(err, "bad url %s for %s", urlStr, commandName)
		}
		return u, nil
	}
	if commandName == SendButtonsCmd {
		buttons, err := bp.parseButtonsArg(args)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse %s command args to buttons", SendButtonsCmd)
		}
		return buttons, nil
	}
	// commands with named "buttons" param, e.g send_text_with_buttons
	argsAsObject, ok := args.(map[string]interface{})
	if !ok {
		return args, nil
	}
	buttonsArg := argsAsObject["buttons"]
	if buttonsArg == nil {
		return args, nil
	}
	buttons, err := bp.parseButtonsArg(buttonsArg)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse buttons arg of the %s command to buttons", SendButtonsCmd)
	}
	argsWithButtons := make(map[string]interface{}, len(argsAsObject))
	for k, v := range argsAsObject {
		argsWithButtons[k] = v
	}
	argsWithButtons["buttons"] = buttons
	return argsWithButtons, nil
}

func (bp *BasePage) parseURL(rawurl string) (*core.URL, error) {
	u, err := core.NewURLFromStr(rawurl)
	if err != nil {
		return nil, err
	}
	if u.IsRelative() {
		u.Page = bp.Name
	}
	return u, nil
}

func (bp *BasePage) executeScript(req *core.Request, script []*Command) error {
	if len(script) == 0 {
		return nil
	}
	iter := NewIterator(req, bp, script, bp.messenger)
	err := iter.Run()
	return errors.Wrap(err, "script iteration failed")
}

func (bp *BasePage) GetRequestAction(req *core.Request) string {
	if req.URL.Action != "" {
		return req.URL.Action
	}
	return bp.entryAction
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
			err := templ.Must(templ.New("arg").Option("missingkey=zero").Parse(textArg)).Execute(&b, scriptData)
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
	ifArg, ok := item["if"]
	if !ok {
		return nil, errors.New("if key doesn't present")
	}
	var condition bool
	var isOperatorPresent bool
	for _, operator := range []string{"eq", "ne"} {
		operatorArg, ok := item[operator]
		if ok {
			isEqual := reflect.DeepEqual(ifArg, operatorArg)
			if operator == "eq" {
				condition = isEqual
			} else {
				condition = !isEqual
			}
			isOperatorPresent = true
			break
		}
	}
	if !isOperatorPresent {
		var ok bool
		if ifArg == nil {
			condition = false
		} else {
			condition, ok = ifArg.(bool)
			if !ok {
				var err error
				stringArg, _ := ifArg.(string)
				condition, err = strconv.ParseBool(stringArg)
				if err != nil {
					condition = !reflect.DeepEqual(ifArg, reflect.Zero(reflect.TypeOf(ifArg)).Interface())
				}
			}
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
			value = obj[key]
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
	// priority common<global<action
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

func BadInputResponse(errorMsg string) (map[string]interface{}, *core.URL, error) {
	return map[string]interface{}{"error": true, "error_msg": errorMsg}, nil, nil
}
