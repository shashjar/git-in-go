package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readPktLines(reader io.Reader) ([]string, error) {
	pktLines := []string{}
	bufReader := bufio.NewReader(reader)
	passedStart := false

	for {
		lengthHex := make([]byte, 4)
		_, err := io.ReadFull(bufReader, lengthHex)
		if err != nil {
			return nil, fmt.Errorf("failed to read pkt-line length: %v", err)
		}

		length, err := strconv.ParseInt(string(lengthHex), 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid pkt-line length: %v", err)
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
