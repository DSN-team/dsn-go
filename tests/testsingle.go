package main

import (
	"fmt"
	"github.com/DSN-team/core"
	"github.com/DSN-team/core/tests/utils"
	utils2 "github.com/DSN-team/core/utils"
	"log"
	"time"
)

func main() {
	utils.InitTest()
	profile0 := utils.RunProfile("0")
	profile1 := utils.RunProfile("1")
	utils.CreateNetwork(profile0, profile1)
	utils.CreateNetwork(profile1, profile0)
	go utils.StartConnection(profile0)
	go utils.StartConnection(profile1)

	delayedCall(profile0, profile1, "test")

	//Hold main thread
	for {
		time.Sleep(10)
	}
}

func delayedCall(from, to *core.Profile, msg string) {
	for i := 0; i < 2; i++ {
		go func() {
			time.Sleep(250 * time.Millisecond)
			fmt.Println(utils.ConnectionsToString(from))
			for i := 0; i < len(msg); i++ {
				from.DataStrInput[i] = msg[i]
			}
			core.UpdateUI = func(i int, client int) {
				log.Print("client:", client, "\n")
				log.Print("got it:", to.DataStrOutput)
				log.Println("got it as string:", string(to.DataStrOutput))
			}
			request := from.BuildDataRequest(utils2.RequestData, uint64(len(msg)), from.DataStrInput[0:len(msg)], from.Friends[0].ID)
			fmt.Println("REQUEST:", request)
			from.WriteRequest(int(from.Friends[0].ID), request)
		}()
	}
}
