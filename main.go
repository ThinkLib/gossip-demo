package main

import (
	"time"

	"github.com/ghettovoice/gossip/log"
)

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
	log.SetDefaultLogLevel(log.DEBUG)
	err := caller.Start()
	if err != nil {
		log.Warn("Failed to start caller: %v", err)
		return
	}

	// Receive an incoming call.
	// caller.ServeInvite()
	caller.Invite(callee)

	<-time.After(2 * time.Second)

	// Send the BYE
	caller.Bye(callee)
}
