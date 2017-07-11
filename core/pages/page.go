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
	Enter(req *core.Request, params map[string]interface{}) (string, error)
}

type SequenceItem struct {
	Key   string
	Value interface{}
}

type Controller func(req *core.Request, params map[string]interface{}) (interface{}, error)

type PageStructure struct {
	Main    []map[string]interface{}            `yaml:"main"`
	Intents []*core.Intent                      `yaml:"intents"`
	Parts   map[string][]map[string]interface{} `yaml:"parts"`
	Config  map[string]interface{}              `yaml:"config"`
}

type BasePage struct {
	*logging.GlobalLogger
	Name          string
	messenger     messenger.Messenger
	Controller    Controller
	InputHandlers map[string]MessageHandler

	ParsedPage *PageStructure
	Intents    []*core.Intent
	OtherParts map[string][]*SequenceItem
	MainPart   []*SequenceItem
}

func newBasePage(name string, inputHandlers map[string]MessageHandler, controller Controller,
	messenger messenger.Messenger) (*BasePage, error) {

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
	mainPart, err := retrieveMainPart(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build main part")
	}
	otherParts, err := retrieveOtherParts(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve other parts")
	}
	logger := logging.NewGlobalLogger("pages", log.Fields{"page": name})
	page := &BasePage{Name: name, messenger: messenger, Controller: controller, InputHandlers: inputHandlers,
		ParsedPage: parsedPage, Intents: parsedPage.Intents, MainPart: mainPart, OtherParts: otherParts, GlobalLogger: logger}
	return page, nil
}

func retrieveMainPart(parsedPage *PageStructure) ([]*SequenceItem, error) {
	return iterationPartToSequence(parsedPage.Main)
}

func retrieveOtherParts(parsedPage *PageStructure) (map[string][]*SequenceItem, error) {
	parts := make(map[string][]*SequenceItem, len(parsedPage.Parts))
	for partName, partData := range parsedPage.Parts {
		seq, err := iterationPartToSequence(partData)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot transform part %s to sequence", partName)
		}
		parts[partName] = seq
	}
	return parts, nil
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

func (bp *BasePage) Enter(req *core.Request, params map[string]interface{}) (string, error) {
	var scriptData interface{}
	if bp.Controller != nil {
		var err error
		scriptData, err = bp.Controller(req, params)
		if err != nil {
			return "", errors.Wrap(err, "controller failed")
		}
	}
	redirectURI, err := bp.renderResponse(req, scriptData)
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

func (bp *BasePage) partNames() []string {
	names := make([]string, 0, len(bp.OtherParts))
	for k := range bp.OtherParts {
		names = append(names, k)
	}
	return names
}

func (bp *BasePage) renderResponse(req *core.Request, data interface{}) (string, error) {
	nextPart := bp.MainPart
	var script []*iterator.Command
	var redirectURI string
	for nextPart != nil {
		currentPart := nextPart
		nextPart = nil
		for _, item := range currentPart {
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
				partName, ok := cmd.Args.(string)
				if !ok {
					return "", errors.Errorf("goto argument must be a string, not %v", cmd.Args)
				}
				nextPart, ok = bp.OtherParts[partName]
				if !ok {
					return "", errors.Errorf("goto to unexisting page part %s, parts=%v", partName, bp.partNames())
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

func evaluateArgs(args interface{}, scriptData interface{}) (interface{}, error) {
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
	lookupFields := strings.Split(dataKey, ".")
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

		} else {
			key := field
			obj, ok := value.(map[string]interface{})
			if !ok {
				return nil, errors.Errorf("%v not a json object, lookup key=%s", value, key)
			}
			value, ok = obj[key]
			if !ok {
				return nil, errors.Errorf("key %s not found in %v", key, obj)
			}
		}
	}
	return value, nil
}

func NewHomePage(messenger messenger.Messenger) (*BasePage, error) {
	return newBasePage("home", nil, nil, messenger)
}

type ReminderListPage struct {
	*BasePage
	PreviewTemplate string
}

func (rl *ReminderListPage) getOrDeleteInputHandler(req *core.Request) (string, error) {
	return "", nil
}

func (rl *ReminderListPage) controller(req *core.Request, params map[string]interface{}) (interface{}, error) {
	data := map[string]interface{}{
		"has_reminders":     true,
		"reminder_previews": []interface{}{"foo", "bbbbbb", "222"},
		"foo":               map[string]interface{}{"bar": []interface{}{2, 3, "4"}},
	}

	return data, nil
}

func NewReminderListPage(messenger messenger.Messenger) (*ReminderListPage, error) {
	page := new(ReminderListPage)
	inputs := map[string]MessageHandler{
		"on_get_or_delete": page.getOrDeleteInputHandler,
	}
	basePage, err := newBasePage("reminder_list", inputs, page.controller, messenger)
	if err != nil {
		return nil, err
	}
	previewTemplate, _ := basePage.ParsedPage.Config["preview_template"].(string)
	if len(previewTemplate) == 0 {
		return nil, errors.Errorf("config doesn't contain preview template %v", basePage.ParsedPage.Config)
	}
	return &ReminderListPage{BasePage: basePage, PreviewTemplate: previewTemplate}, nil
}
