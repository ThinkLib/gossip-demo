package main

import (
	"flag"
	"time"

	"github.com/ghettovoice/gossip/log"
)

var cmdFlag = flag.String("cmd", "", "run cmd: [caller, callee]")

var (
	// Caller parameters
	caller = &endpoint{
		displayName: "Ryan",
		username:    "ryan",
		host:        "localhost",
		port:        5070,
		transport:   "UDP",
	}

	// Callee parameters
	callee = &endpoint{
		displayName: "Ryan's PC",
		username:    "stefan",
		host:        "localhost",
		port:        5060,
		transport:   "UDP",
	}
)

func main() {
	flag.Parse()
	log.SetDefaultLogLevel(log.DEBUG)

	switch *cmdFlag {
	case "caller":
		err := caller.Start()
		if err != nil {
			log.Warn("Failed to start caller: %v", err)
			return
		}
		caller.Invite(callee)
		<-time.After(2 * time.Second)
		caller.Bye(callee)
	case "callee":
		err := callee.Start()
		if err != nil {
			log.Warn("Failed to start callee: %v", err)
			return
		}
		callee.Serve()
	default:
		log.Error("unknown command")
	}
}
