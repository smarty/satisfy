package main

import "os"

// environmentVariables looks up environment variables (os.LookupEnv in
// production, a map in tests).
type environmentVariables interface {
	LookupEnv(key string) (value string, set bool)
}

type osEnvironment struct{}

func newOSEnvironment() osEnvironment { return osEnvironment{} }

func (osEnvironment) LookupEnv(key string) (string, bool) { return os.LookupEnv(key) }
