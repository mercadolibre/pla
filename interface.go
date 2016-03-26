package main

import "github.com/sschepens/pla/boomer"

type Interface interface {
	Start(b *boomer.Boomer)
	ProcessResult(res boomer.Result)
	End()
}
