package driver

import "fmt"

type Factory func(cfg ConnectionConfig) (Connector, Backuper, Restorer)

var registry = map[string]Factory{}

func Register(engine string, factory Factory) {
	if _, exists := registry[engine]; exists {
		panic(fmt.Sprintf("driver: engine %q already registered", engine))
	}
	registry[engine] = factory
}

func Get(engine string) (Factory, bool) {
	factory, ok := registry[engine]
	return factory, ok
}