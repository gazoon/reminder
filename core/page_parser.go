package core

import (
	"bytes"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"strconv"
	"strings"
	templ "text/template"
)

const (
	evaluationMarker = "$"
)

func parseYAML(data []byte) ([]map[string]interface{}, error) {
	parsed := &[]map[string]interface{}{}
	err := yaml.Unmarshal(data, parsed)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func executeTemplate(template string, templateData interface{}) ([]byte, error) {
	b := bytes.Buffer{}
	err := templ.Must(templ.New("page").Parse(template)).Execute(&b, templateData)
	if err != nil {
		return nil, errors.Wrap(err, "template execute failed")
	}
	return b.Bytes(), nil
}

type PageParser struct {
	parseContent func(data []byte) ([]map[string]interface{}, error)
}

func (p *PageParser) ToScript(template string, templateData, scriptData interface{}) ([]*Command, error) {
	content, err := executeTemplate(template, templateData)
	if err != nil {
		return nil, err
	}
	parsed, err := p.parseContent(content)
	if err != nil {
		return nil, errors.Wrap(err, "content parsing failed")
	}
	return parsedPageToScript(parsed, scriptData)
}

func parsedPageToScript(parsed []map[string]interface{}, scriptData interface{}) ([]*Command, error) {
	script := make([]*Command, len(parsed))
	for idx, item := range parsed {
		if len(item) != 1 {
			return nil, errors.Errorf("script command item must have one key-value pair %s", item)
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
			evaluatedValue = args
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

func retrieveValue(dataKey string, scriptData interface{}) (interface{}, error) {
	lookupFields := strings.Split(dataKey, ".")
	var value interface{} = scriptData
	for fieldNum, field := range lookupFields {
		if index, err := strconv.Atoi(field); err != nil {
			array, ok := value.([]interface{})
			if !ok {
				return nil, errors.Errorf("%v not an array, lookup field number=%s field=%s", value, fieldNum, field)
			}
			value = array[index]

		} else {
			key := field
			obj, ok := value.(map[string]interface{})
			if !ok {
				return nil, errors.Errorf("%v not a json object, lookup field number=%s field=%s", value, fieldNum, field)
			}
			value = obj[key]
		}
	}
	return value, nil
}
