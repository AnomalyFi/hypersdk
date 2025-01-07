package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
func sendJSONRPCRequest(url string, method string, params interface{}, id int) (*JSONRPCResponse, error) { //nolint:unparam
	var client http.Client
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	responseBytes, err := io.ReadAll(resp.Body)
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
	_, err := sendJSONRPCRequest(cli.uri, fmt.Sprintf("%s.%s", valSerName, "UpdateArcadiaURL"), []interface{}{UpdateArcadiaURLArgs{URL: url}}, 1)
	return err
}
