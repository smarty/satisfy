package shell

import "os"

type Environment struct{}

func NewEnvironment() *Environment {
	return &Environment{}
}

func (this *Environment) LookupEnv(key string) (value string, set bool) {
	return os.LookupEnv(key)
}
