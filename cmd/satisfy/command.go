package main

type Command struct {
	Name string

	Description string
	Usage       string

	Function func()
}
