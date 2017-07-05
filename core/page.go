package core

type RequestHandler func(req *Request) (string, error)
type Page interface {
	GetIntents() []*Intent
	GetInputHandler(name string) (RequestHandler, bool)
	Serve(req *Request, params map[string]interface{}) ([]*Command, error)
}

type SequenceItem struct {
	Key string
	Value interface{}
}

type basePage struct {
	name string
	intents       []*Intent
	inputHandlers map[string]RequestHandler
	parsedPage map[string]interface{}
	parts map[string][]*SequenceItem
	mainPart []*SequenceItem

}

func (bp *basePage) GetIntents() []*Intent {
	return bp.intents
}

func (bp *basePage) GetInputHandler(name string) (RequestHandler, bool) {
	return bp.inputHandlers[name]
}
