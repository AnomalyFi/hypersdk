package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sendJSONRPCRequest sends a JSON-RPC request and returns the response or an error
func sendJSONRPCRequest(url string, method string, params interface{}, id int) (*JSONRPCResponse, error) {
	// Create the JSON-RPC request object
	requestBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	// Marshal the request into JSON
	requestBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send the HTTP POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response JSONRPCResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		fmt.Println(string(responseBytes))
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

type JSONRPCValClient struct {
	uri string
}

func NewJSONRPCValClient(uri string) *JSONRPCValClient {
	uri = strings.TrimSuffix(uri, "/")
	return &JSONRPCValClient{uri: uri}
}

func (cli *JSONRPCValClient) Ping() (bool, error) {
	_, err := sendJSONRPCRequest(cli.uri, fmt.Sprintf("%s.%s", valSerName, "Ping"), []interface{}{}, 1)
	return true, err
}

func (cli *JSONRPCValClient) UpdateArcadiaURL(url string) error {
	_, err := sendJSONRPCRequest(cli.uri, fmt.Sprintf("%s.%s", valSerName, "UpdateArcadiaURL"), []interface{}{UpdateArcadiaURLArgs{Url: url}}, 1)
	return err
}
