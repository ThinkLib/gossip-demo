package main

import (
	"fmt"
	"time"

	"github.com/ghettovoice/gossip/base"
	"github.com/ghettovoice/gossip/log"
	"github.com/ghettovoice/gossip/transaction"
	"github.com/ghettovoice/gossip/transport"
)

type endpoint struct {
	// Sip Params
	displayName string
	username    string
	host        string

	// Transport Params
	port      uint16 // Listens on this port.
	transport string // Sends using this transport. ("tcp" or "udp")

	// Internal guts
	tm       *transaction.Manager
	dialog   dialog
	dialogIx int
}

type dialog struct {
	callId    string
	to_tag    string // The tag in the To header.
	from_tag  string // The tag in the From header.
	currentTx txInfo // The current transaction.
	cseq      uint32
}

type txInfo struct {
	tx     transaction.Transaction // The underlying transaction.
	branch string                  // The via branch.
}

func (e *endpoint) Start() error {
	tp, err := transport.NewManager(e.transport)
	tm, err := transaction.NewManager(tp, fmt.Sprintf("%v:%v", e.host, e.port))
	if err != nil {
		return err
	}

	e.tm = tm

	return nil
}

func (e *endpoint) ClearDialog() {
	caller.dialog = dialog{}
}

func (caller *endpoint) Invite(callee *endpoint) error {
	// Starting a dialog.
	callid := "thisisacall"
	tag := "tag." + caller.username + "." + caller.host
	branch := "z9hG4bK.callbranch.INVITE"
	caller.dialog.callId = callid
	caller.dialog.from_tag = tag
	caller.dialog.currentTx = txInfo{}
	caller.dialog.currentTx.branch = branch

	ctx, cancel := base.NewContext()
	defer cancel()
	invite := base.NewRequest(
		ctx,
		base.INVITE,
		&base.SipUri{
			User: base.String{S: callee.username},
			Host: callee.host,
		},
		"SIP/2.0",
		[]base.SipHeader{
			Via(caller, branch),
			To(callee, caller.dialog.to_tag),
			From(caller, caller.dialog.from_tag),
			Contact(caller),
			CSeq(caller.dialog.cseq, base.INVITE),
			CallId(callid),
			ContentLength(0),
		},
		"",
	)
	caller.dialog.cseq++

	invite.Log().Infof("Sending: %v", invite.Short())
	tx := caller.tm.Send(invite, fmt.Sprintf("%v:%v", callee.host, callee.port))
	caller.dialog.currentTx.tx = transaction.Transaction(tx)
	for {
		select {
		case r := <-tx.Responses():
			// Get To tag if present.
			tag, ok := r.Headers("To")[0].(*base.ToHeader).Params.Get("tag")
			if ok {
				caller.dialog.to_tag = tag.(*base.String).S
			}

			switch {
			case r.StatusCode >= 300:
				// Call setup failed.
				return fmt.Errorf("callee sent negative response code %v", r.StatusCode)
			case r.StatusCode >= 200:
				<-time.After(1 * time.Second)
				// Ack 200s manually.
				r.Log().Info("Sending Ack")
				tx.Ack()
				return nil
			}
		case e := <-tx.Errors():
			tx.Log().Warn(e.Error())
			return e
		}
	}
}

func (caller *endpoint) Bye(callee *endpoint) error {
	return caller.nonInvite(callee, base.BYE)
}

func (caller *endpoint) nonInvite(callee *endpoint, method base.Method) error {
	caller.dialog.currentTx.branch = "z9hG4bK.callbranch." + string(method)
	ctx, cancel := base.NewContext()
	defer cancel()
	request := base.NewRequest(
		ctx,
		method,
		&base.SipUri{
			User: base.String{S: callee.username},
			Host: callee.host,
		},
		"SIP/2.0",
		[]base.SipHeader{
			Via(caller, caller.dialog.currentTx.branch),
			To(callee, caller.dialog.to_tag),
			From(caller, caller.dialog.from_tag),
			Contact(caller),
			CSeq(caller.dialog.cseq, method),
			CallId(caller.dialog.callId),
			ContentLength(0),
		},
		"",
	)
	caller.dialog.cseq++

	request.Log().Infof("Sending: %v", request.Short())
	tx := caller.tm.Send(request, fmt.Sprintf("%v:%v", callee.host, callee.port))
	caller.dialog.currentTx.tx = transaction.Transaction(tx)
	for {
		select {
		case r := <-tx.Responses():
			switch {
			case r.StatusCode >= 300:
				// Failure (or redirect).
				return fmt.Errorf("callee sent negative response code %v", r.StatusCode)
			case r.StatusCode >= 200:
				// Success.
				r.Log().Info("Successful transaction")
				return nil
			}
		case e := <-tx.Errors():
			tx.Log().Warn(e.Error())
			return e
		}
	}
}

// Server side function.

func (e *endpoint) Serve() {
	log.Info("Listening for incoming requests...")

	for tx := range e.tm.Requests() {
		r := tx.Origin()

		e.dialog.callId = string(*r.Headers("Call-Id")[0].(*base.CallId))

		// Send a 200 OK
		resp := base.NewResponse(
			r.Context(),
			"SIP/2.0",
			200,
			"OK",
			[]base.SipHeader{},
			"",
		)

		base.CopyHeaders("Via", tx.Origin(), resp)
		base.CopyHeaders("From", tx.Origin(), resp)
		base.CopyHeaders("To", tx.Origin(), resp)
		base.CopyHeaders("Call-Id", tx.Origin(), resp)
		base.CopyHeaders("CSeq", tx.Origin(), resp)
		resp.AddHeader(
			&base.ContactHeader{
				DisplayName: base.String{S: e.displayName},
				Address: &base.SipUri{
					User: base.String{S: e.username},
					Host: e.host,
				},
			},
		)

		<-time.After(1 * time.Second)
		tx.Log().Info("Sending 200 OK")
		tx.Respond(resp)

		// await ACK on INVITE transaction only if response is >= 2xx
		// if r.Method == base.INVITE {
		// 	tx.Log().Warnf("Await for ACK request")
		// 	ack := <-tx.Ack()
		// 	ack.Log().Warnf("Received ACK %s", ack.Short())
		// }
	}
}
