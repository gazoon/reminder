package core

import (
	"bytes"
	"github.com/gazoon/bot_libs/messenger"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	templ "text/template"
)

const (
	fileExtension    = ".yaml"
	pagesFolder      = "pages"
	evaluationMarker = "$"
)

var (
	fileContentParser = parseYAML
)

func parseYAML(data []byte) (map[string]interface{}, error) {
	parsed := &map[string]interface{}{}
	err := yaml.Unmarshal(data, parsed)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

type MessageHandler func(req *Request) (string, error)

type Page interface {
	GetIntents() []*Intent
	GetInputHandler(name string) (MessageHandler, bool)
	Enter(req *Request, params map[string]interface{}) (string, error)
}

type SequenceItem struct {
	Key   string
	Value interface{}
}

type Controller func(req *Request, params map[string]interface{}) (interface{}, error)

type PageStructure struct {
	Main    []map[string]interface{}            `mapstructure:"main"`
	Intents []*Intent                           `mapstructure:"intents"`
	Parts   map[string][]map[string]interface{} `mapstructure:"parts"`
	Config  map[string]interface{}              `mapstructure:"config"`
}

type BasePage struct {
	name          string
	messenger     messenger.Messenger
	controller    Controller
	inputHandlers map[string]MessageHandler

	parsedPage *PageStructure
	intents []*Intent
	otherParts map[string][]*SequenceItem
	mainPart []*SequenceItem

}

func newBasePage(name string, inputHandlers map[string]MessageHandler, controller Controller,
	messenger messenger.Messenger) (*BasePage, error) {

	filePath := path.Join(pagesFolder, name+fileExtension)
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "read page content")
	}
	parsedPage, err := fileContentParser(fileContent)
	if err != nil {
		return nil, errors.Wrap(err, "content parsing failed")
	}
	intents, err := retrieveIntents(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build intents")
	}
	mainPart, err := retrieveMainPart(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot build main part")
	}
	otherParts, err := retrieveOtherParts(parsedPage)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve other parts")
	}
	page := &BasePage{name: name, messenger: messenger, controller: controller, inputHandlers: inputHandlers,
		parsedPage: parsedPage, intents: intents, mainPart: mainPart, otherParts: otherParts}
	return page, nil
}

func retrieveIntents(parsedPage map[string]interface{}) ([]*Intent, error) {
	intentsSection, ok := parsedPage["intents"]
	if !ok {
		return nil, nil
	}
	intentsArray, ok := intentsSection.([]interface{})
	if !ok {
		return nil, errors.Errorf("intents must be an array, not %v", intentsSection)
	}
	intents := make([]*Intent, len(intentsArray))
	for i, intentData := range intentsArray {
		intent := new(Intent)
		err := mapstructure.Decode(intentData, intent)
		if err != nil {
			return nil, errors.Wrapf(err, "fail to transform intent data %v to intent", intentData)
		}
		intents[i] = intent
	}
	return intents, nil
}

func retrieveMainPart(parsedPage map[string]interface{}) ([]*SequenceItem, error) {
	mainSection, ok := parsedPage["main"]
	if !ok {
		return nil, errors.New("no main part")
	}
	return iterationPartToSequence(mainSection)
}

func retrieveOtherParts(parsedPage map[string]interface{}) (map[string][]*SequenceItem, error) {
	partsSection, ok := parsedPage["parts"]
	if !ok {
		return nil, nil
	}
	partsData, ok := partsSection.(map[string]interface{})
	parts := make(map[string][]*SequenceItem, len(partsData))
	for partName, partData := range partsData {
		seq, err := iterationPartToSequence(partData)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot transform part %s to sequence", partName)
		}
		parts[partName] = seq
	}
	return parts, nil
}

func iterationPartToSequence(iterationPart interface{}) ([]*SequenceItem, error) {
	data, ok := iterationPart.([]map[string]interface{})
	if !ok {
		return nil, errors.Errorf("iteration part must be an array of objects %v", iterationPart)
	}
	script := make([]*Command, len(data))
	for idx, item := range data {
		if len(item) != 1 {
			return nil, errors.Errorf("sequence item must have one key-value pair %s", item)
		}
		cmd := &Command{}
		for name, args := range item {
			// there is only one cycle
			cmd.Name = name
			cmd.Args = args
		}
		script[idx] = cmd
	}
	return script, nil
}

func (bp *BasePage) Enter(req *Request, params map[string]interface{}) (string, error) {
	var scriptData interface{}
	if bp.controller != nil {
		var err error
		scriptData, err = bp.controller(req, params)
		if err != nil {
			return "", errors.Wrap(err, "controller failed")
		}
	}
	redirectURI, err := bp.renderResponse(req, scriptData)
	return redirectURI, errors.Wrap(err, "response failed")
}

func (bp *BasePage) GetIntents() []*Intent {
	return bp.intents
}

func (bp *BasePage) GetInputHandler(name string) (MessageHandler, bool) {
	return bp.inputHandlers[name]
}

func (bp *BasePage) partNames() []string {
	names := make([]string, 0, len(bp.otherParts))
	for k := range bp.otherParts {
		names = append(names, k)
	}
	return names
}

func (bp *BasePage) renderResponse(req *Request, data interface{}) (string, error) {
	nextPart := bp.mainPart
	var script []*Command
	var redirectURI string
	for nextPart != nil {
		currentPart := nextPart
		nextPart = nil
		for _, item := range currentPart {
			var err error
			cmd := &Command{Name: item.Key}
			cmd.Args, err = evaluateArgs(item.Value, data)
			if err != nil {
				return "", errors.Wrapf(err, "args evaluation failed, args=%v data=%v command=%s", item.Value, data, cmd.Name)
			}
			if cmd.Name == "goto" {
				if cmd.Args == nil {
					continue
				}
				partName, ok := cmd.Args.(string)
				if !ok {
					return "", errors.Errorf("goto argument must be a string, not %v", cmd.Args)
				}
				nextPart, ok = bp.otherParts[partName]
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

func (bp *BasePage) executeScript(req *Request, script []*Command) error {
	if len(script) == 0 {
		return nil
	}
	iter := NewIterator(req, script, bp.messenger)
	err := iter.Run()
	return errors.Wrap(err, "script iteration falied")
}

func (bp *BasePage) fillConfig(config interface{}) error {
	configData := bp.parsedPage["config"]
	err := mapstructure.Decode(configData, config)
	if err != nil {
		return errors.Wrapf(err, "fail to transform config data %v to %T", configData, config)
	}
	return nil
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
		var err error
		evaluatedValue, err = evaluateConditionalValue(evaluatedObject)
		if err != nil {
			return nil, errors.Wrap(err, "cannot evaluate cond value")
		}
	} else {
		evaluatedValue = args
	}
	return evaluatedValue, nil
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

func evaluateConditionalValue(data map[string]interface{}) (interface{}, error) {
	value, err := ifStatement(data)
	if err == nil {
		return value
	}
	condArg := data["cond"]
	if condArg == nil {
		return data, nil
	}
	ifSequence, ok := condArg.([]interface{})
	if !ok {
		return nil, errors.Errorf("cond arg must be an array: %v", condArg)
	}
	for _, ifData := range ifSequence {
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
		if index, err := strconv.Atoi(field); err != nil {
			array, ok := value.([]interface{})
			if !ok {
				return nil, errors.Errorf("%v not an array, lookup index=%s", value, index)
			}
			if index < 0 || index >= len(array) {
				return nil, errors.Errorf("index %s out of range %v", index, value)
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
				return nil, errors.Errorf("key %s not found in %v", key, value)
			}
		}
	}
	return value, nil
}

func NewHomePage(messenger messenger.Messenger) (*BasePage, error) {
	return newBasePage("home", nil, nil, messenger)
}

type ReminderListConfig struct {
	PreviewTemplate string
}

type ReminderListPage struct {
	*BasePage
	config *ReminderListConfig
}

func NewReminderListPage(messenger.Messenger) {

}
