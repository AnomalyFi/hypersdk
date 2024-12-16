package rpc

import (
	"context"
	"strings"

	"github.com/AnomalyFi/hypersdk/requester"
)

type JSONRPCValClient struct {
	requester *requester.EndpointRequester
}

func NewJSONRPCValClient(uri string) *JSONRPCValClient {
	uri = strings.TrimSuffix(uri, "/")
	req := requester.New(uri, valSerName)
	return &JSONRPCValClient{requester: req}
}

func (cli *JSONRPCValClient) Ping(ctx context.Context) (bool, error) {
	resp := new(PingReply)
	err := cli.requester.SendRequest(ctx,
		"Ping",
		nil,
		resp,
	)
	return resp.Success, err
}

func (cli *JSONRPCValClient) UpdateArcadiaURL(ctx context.Context, url string) error {
	return cli.requester.SendRequest(
		ctx,
		"UpdateArcadiaURL",
		&UpdateArcadiaURLArgs{Url: url},
		nil,
	)
}

// func (cli *JSONRPCValClient) SetMempoolExemptSponsors(ctx context.Context, )
