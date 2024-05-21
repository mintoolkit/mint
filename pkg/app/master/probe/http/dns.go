package http

import (
	"context"
	"fmt"

	"github.com/miekg/dns"

	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

const (
	defaultDNSPortStr = "53"
)

var dnsReqBindVersion = &dns.Msg{
	MsgHdr: dns.MsgHdr{
		Id:               dns.Id(),
		RecursionDesired: true,
	},
	Question: []dns.Question{
		{
			Name:   dns.Fqdn("version.bind"),
			Qtype:  dns.TypeTXT,
			Qclass: dns.ClassCHAOS,
		},
	},
}

var dnsReqWhoami = &dns.Msg{
	MsgHdr: dns.MsgHdr{
		Id:               dns.Id(),
		RecursionDesired: true,
	},
	Question: []dns.Question{
		{
			Name:   dns.Fqdn("whoami.example.org"),
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		},
	},
}

// all DNS servers should respond to the NS query for the root zone
var dnsReqRootNS = &dns.Msg{
	MsgHdr: dns.MsgHdr{
		Id:               dns.Id(),
		RecursionDesired: true,
	},
	Question: []dns.Question{
		{
			Name:   dns.Fqdn("."),
			Qtype:  dns.TypeNS,
			Qclass: dns.ClassINET,
		},
	},
}

func dnsPing(ctx context.Context, targetHost string, port string, useTCP bool) (string, error) {
	addr := fmt.Sprintf("%s:%s", targetHost, port)
	client := &dns.Client{}

	if useTCP {
		client.Net = "tcp"
	}

	resp, _, err := client.ExchangeContext(ctx, dnsReqRootNS, addr)
	if err != nil {
		return "", err
	}

	return jsonutil.ToString(resp), nil
}
