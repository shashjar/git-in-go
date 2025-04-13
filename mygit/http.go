package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
)

func makeHTTPRequest(method string, url string, body bytes.Buffer, expectedStatusCodes []int) ([]byte, error) {
	username := os.Getenv("GIT_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("GIT_USERNAME environment variable not set")
	}

	token := os.Getenv("GIT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GIT_TOKEN environment variable not set. Please create a personal access token at https://github.com/settings/tokens")
	}

	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request to %s with method %s: %s", url, method, err)
	}

	req.SetBasicAuth(username, token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request to %s with method %s failed: %s", url, method, err)
	}
	defer resp.Body.Close()

	receivedExpectedStatusCode := slices.Contains(expectedStatusCodes, resp.StatusCode)
	if !receivedExpectedStatusCode {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("received invalid response status code %s for HTTP request to %s with method %s. Response body: %s", resp.Status, url, method, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body for HTTP request to %s with method %s: %s", url, method, err)
	}

	return respBody, nil
}
