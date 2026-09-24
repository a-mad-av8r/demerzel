package accountscli

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
)

type listedAccount struct {
	Group      group
	Credential credentialItem
}

func writeAccountTable(output io.Writer, accounts []listedAccount) error {
	if len(accounts) == 0 {
		_, err := fmt.Fprintln(output, "No accounts found.")
		return err
	}
	writer := tabwriter.NewWriter(output, 0, 8, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "GROUP_ID\tGROUP\tCREDENTIAL_ID\tLABEL\tACCOUNT\tSTATUS\tQUOTA\tCOOLDOWN\tHEALTH"); err != nil {
		return err
	}
	for _, account := range accounts {
		item := account.Credential
		if _, err := fmt.Fprintf(writer, "%d\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			account.Group.ID,
			safeDisplay(account.Group.Name),
			item.CredentialID,
			fallback(item.Label, "-"),
			accountDisplay(item),
			statusDisplay(item),
			quotaDisplay(item.Observation),
			cooldownDisplay(item),
			healthDisplay(item),
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func accountDisplay(item credentialItem) string {
	if item.Account.Email != "" {
		return safeDisplay(item.Account.Email)
	}
	if item.Observation != nil && item.Observation.Snapshot != nil && item.Observation.Snapshot.Account != nil {
		account := item.Observation.Snapshot.Account
		if account.Email != "" {
			return safeDisplay(account.Email)
		}
		if account.DisplayName != "" {
			return safeDisplay(account.DisplayName)
		}
	}
	if item.Account.EmailMask != "" {
		return safeDisplay(item.Account.EmailMask)
	}
	return fallback(item.Mask, "-")
}

func statusDisplay(item credentialItem) string {
	configured := fallback(item.ConfiguredStatus, "unknown")
	effective := fallback(item.EffectiveStatus, "unknown")
	return safeDisplay(configured + "/" + effective)
}

func quotaDisplay(value *observation) string {
	if value == nil {
		return "not reported"
	}
	if value.Snapshot == nil {
		return fallback(safeDisplay(value.State), "unavailable")
	}
	if len(value.Snapshot.QuotaWindows) == 0 {
		return "none reported"
	}
	windows := make([]string, 0, len(value.Snapshot.QuotaWindows))
	for _, window := range value.Snapshot.QuotaWindows {
		name := fallback(window.Label, window.ID)
		parts := make([]string, 0, 3)
		if window.Remaining != nil {
			parts = append(parts, "remaining="+strconv.FormatFloat(*window.Remaining, 'f', -1, 64))
		}
		if window.Limit != nil {
			parts = append(parts, "limit="+strconv.FormatFloat(*window.Limit, 'f', -1, 64))
		}
		if window.Unit != "" {
			parts = append(parts, "unit="+safeDisplay(window.Unit))
		}
		if window.State != "" {
			parts = append(parts, "state="+safeDisplay(window.State))
		}
		if window.ResetAtMS != nil {
			parts = append(parts, "reset="+time.UnixMilli(*window.ResetAtMS).UTC().Format(time.RFC3339))
		}
		if len(parts) == 0 {
			windows = append(windows, safeDisplay(name))
			continue
		}
		windows = append(windows, safeDisplay(name)+"{"+strings.Join(parts, ",")+"}")
	}
	return strings.Join(windows, "; ")
}

func cooldownDisplay(item credentialItem) string {
	if item.CooldownUntilMS != nil {
		return time.UnixMilli(*item.CooldownUntilMS).UTC().Format(time.RFC3339)
	}
	if len(item.ModelCooldowns) == 0 {
		return "-"
	}
	values := make([]string, 0, len(item.ModelCooldowns))
	for _, cooldown := range item.ModelCooldowns {
		values = append(values, safeDisplay(cooldown.Model)+"@"+time.UnixMilli(cooldown.CooldownUntilMS).UTC().Format(time.RFC3339))
	}
	return strings.Join(values, "; ")
}

func healthDisplay(item credentialItem) string {
	parts := make([]string, 0, 5)
	if item.AuthState != "" {
		parts = append(parts, "auth="+safeDisplay(item.AuthState))
	}
	if item.Observation != nil {
		if item.Observation.State != "" {
			parts = append(parts, "observation="+safeDisplay(item.Observation.State))
		}
		if item.Observation.LastErrorCode != "" {
			parts = append(parts, "error="+safeDisplay(item.Observation.LastErrorCode))
		}
	}
	if item.AuthErrorCode != "" {
		parts = append(parts, "auth_error="+safeDisplay(item.AuthErrorCode))
	}
	if item.LastFailureCategory != "" {
		parts = append(parts, "last_failure="+safeDisplay(item.LastFailureCategory))
	}
	if item.RecentSuccessCount > 0 || item.RecentFailureCount > 0 || item.ConsecutiveFailureCount > 0 {
		parts = append(parts, fmt.Sprintf("recent=%d/%d consecutive_failures=%d", item.RecentSuccessCount, item.RecentFailureCount, item.ConsecutiveFailureCount))
	}
	if len(parts) == 0 {
		return "unobserved"
	}
	return strings.Join(parts, ",")
}

func fallback(value, otherwise string) string {
	if value == "" {
		return otherwise
	}
	return safeDisplay(value)
}

func safeDisplay(value string) string {
	var result strings.Builder
	count := 0
	for _, character := range value {
		if count == 128 {
			result.WriteString("...")
			break
		}
		if unicode.IsControl(character) {
			character = ' '
		}
		result.WriteRune(character)
		count++
	}
	return result.String()
}
