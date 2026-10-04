package reservation

import (
	"sort"
	"strings"

	"github.com/google/uuid"
)

// RequestKey fingerprints show + user + seat set for idempotency conflict detection.
func RequestKey(showID uuid.UUID, userID string, seatCodes []string) string {
	sorted := append([]string(nil), seatCodes...)
	sort.Strings(sorted)
	return showID.String() + "|" + userID + "|" + strings.Join(sorted, ",")
}
