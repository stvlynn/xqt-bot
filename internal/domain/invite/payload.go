// Package invite defines the deep-link flow that hands out one-time,
// short-lived group invite links.
package invite

import (
	"fmt"
	"strconv"
	"strings"
)

// PayloadPrefix marks a /start deep-link payload as a join request.
const PayloadPrefix = "j"

// EncodePayload builds the deep-link payload for a target chat,
// e.g. "j-1001234567890".
func EncodePayload(chatID int64) string {
	return PayloadPrefix + strconv.FormatInt(chatID, 10)
}

// ParsePayload extracts the target chat ID from a deep-link payload.
func ParsePayload(payload string) (int64, error) {
	if !strings.HasPrefix(payload, PayloadPrefix) {
		return 0, fmt.Errorf("not a join payload: %q", payload)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(payload, PayloadPrefix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed join payload %q: %w", payload, err)
	}
	return id, nil
}
