package main

import (
	"fmt"
	"io"
	"time"

	"github.com/talostrading/sonic"
)

func main() {
	ioc, err := sonic.NewIO(-1)
	if err != nil {
		panic(err)
	}

	timer, err := sonic.NewTimer(ioc)
	if err != nil {
		panic(err)
	}

	fmt.Println("timer armed: ", time.Now())
	timer.Arm(5*time.Second, func() {
		fmt.Println("timer fired: ", time.Now())
	})

	if err := ioc.RunPending(); err != nil && err != io.EOF {
		panic(err)
	}
}