package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readPktLine(reader io.Reader) (string, error) {
	bufReader := bufio.NewReader(reader)
	lengthHex := make([]byte, 4)
	_, err := io.ReadFull(bufReader, lengthHex)
	if err != nil {
		return "", fmt.Errorf("failed to read pkt-line length: %s", err)
	}

	length, err := strconv.ParseInt(string(lengthHex), 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid pkt-line length: %s", err)
	}

	payloadLength := length - 4
	pktLine := make([]byte, payloadLength)
	n, err := io.ReadFull(bufReader, pktLine)
	if err != nil || int64(n) != payloadLength {
		return "", fmt.Errorf("failed to read pkt-line payload: %s", err)
	}

	pktLinePayload := strings.TrimRight(string(pktLine), "\r\n")
	return pktLinePayload, nil
}

func readPktLines(reader io.Reader) ([]string, error) {
	pktLines := []string{}
	bufReader := bufio.NewReader(reader)
	passedStart := false

	for {
		lengthHex := make([]byte, 4)
		_, err := io.ReadFull(bufReader, lengthHex)
		if err != nil {
			return nil, fmt.Errorf("failed to read pkt-line length: %s", err)
		}

		length, err := strconv.ParseInt(string(lengthHex), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid pkt-line length: %s", err)
		}

		if length == 0 && !passedStart {
			passedStart = true
			continue
		} else if length == 0 {
			break
		}

		payloadLength := length - 4
		pktLine := make([]byte, payloadLength)
		n, err := io.ReadFull(bufReader, pktLine)
		if err != nil || int64(n) != payloadLength {
			return []string{}, fmt.Errorf("failed to read pkt-line payload: %s", err)
		}

		pktLinePayload := strings.TrimRight(string(pktLine), "\r\n")
		pktLines = append(pktLines, pktLinePayload)
	}

	return pktLines, nil
}

func createPktLine(content string) string {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	length := len(content) + 4
	return fmt.Sprintf("%04x%s", length, content)
}

func createPktLineStream(pktLines []string) string {
	var sb strings.Builder
	for _, pktLine := range pktLines {
		sb.WriteString(pktLine)
	}
	if len(pktLines) == 0 || pktLines[len(pktLines)-1] != "0000" {
		sb.WriteString("0000")
	}
	return sb.String()
}
