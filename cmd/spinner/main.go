package main

import (
	"time"

	"github.com/code-gorilla-au/goety/internal/emitter"
	"github.com/code-gorilla-au/goety/internal/spinner"
)

func main() {

	spin := spinner.New(emitter.New())
	defer spin.Stop("")
	spin.Start("starting")
	time.Sleep(1 * time.Second)
	spin.UpdateMessage("first message")
	time.Sleep(1 * time.Second)
	spin.UpdateMessage("foo bar")
	time.Sleep(1 * time.Second)
	spin.UpdateMessage("dis work")
	time.Sleep(1 * time.Second)

}
