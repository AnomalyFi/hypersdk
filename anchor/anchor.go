package anchor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TODO: this file to define anchor client

type AnchorClient struct {
	Url string `json:"url"`
	vm  VM
}

func NewAnchorClient(vm VM, url string) *AnchorClient {
	url = strings.TrimRight(url, "/")
	return &AnchorClient{
		vm:  vm,
		Url: url,
	}
}

// TODO: implement client methods below
func (cli *AnchorClient) GetHeaderV2(slot int64) (*SEQHeaderResponse, error) {
	// slot := 0
	parentHash := "0x0"
	pubkey := "0x0"
	numToBTxs := 5
	numRoBChains := 1
	numRobChunkTxs := 10

	path := fmt.Sprintf("/eth/v1/builder/header2/%d/%s/%s/%d/%d/%d", slot, parentHash, pubkey, numToBTxs, numRoBChains, numRobChunkTxs)
	url := cli.Url + path

	var client http.Client

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var header SEQHeaderResponse
	if err := json.Unmarshal(bodyBytes, &header); err != nil {
		return nil, err
	}

	return &header, nil
}

func (cli *AnchorClient) GetPayloadV2(slot int64) (*SEQPayloadResponse, error) {
	path := "/eth/v1/builder/blinded_blocks2"
	url := cli.Url + path

	req := SEQPayloadRequest{
		Slot: uint64(slot),
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var client http.Client
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload SEQPayloadResponse
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
