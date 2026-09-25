package pricing

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
)

// PriceMultiplier stores a price multiplier in millionths; the zero value means no multiplier.
type PriceMultiplier int64

const DefaultPriceMultiplier PriceMultiplier = 1_000_000

// PriceMultipliers are the Group and AccessKey price multipliers frozen for this request.
type PriceMultipliers struct {
	Group     PriceMultiplier `json:"group"`
	AccessKey PriceMultiplier `json:"access_key"`
}

// ParsePriceMultiplier parses a decimal multiplier with at most six fractional digits in the range 0 to 1000.
func ParsePriceMultiplier(value string) (PriceMultiplier, error) {
	integer, fraction, hasFraction := strings.Cut(value, ".")
	if integer == "" || !decimalDigits(integer) || len(fraction) > 6 ||
		(hasFraction && (fraction == "" || !decimalDigits(fraction))) {
		return 0, errors.New("price multiplier must be a non-negative decimal with at most six decimal places")
	}
	wholeText := strings.TrimLeft(integer, "0")
	if wholeText == "" {
		wholeText = "0"
	}
	whole, err := strconv.ParseInt(wholeText, 10, 64)
	if err != nil || whole > 1000 {
		return 0, errors.New("price multiplier must be between 0 and 1000")
	}
	decimals, err := strconv.ParseInt(fraction+strings.Repeat("0", 6-len(fraction)), 10, 64)
	if err != nil {
		return 0, errors.New("price multiplier has invalid decimal places")
	}
	parsed := PriceMultiplier(whole*int64(DefaultPriceMultiplier) + decimals)
	if !parsed.Valid() {
		return 0, errors.New("price multiplier must be between 0 and 1000")
	}
	return parsed, nil
}

// FormatPriceMultiplier returns the multiplier's canonical decimal representation.
func FormatPriceMultiplier(value PriceMultiplier) string {
	whole := strconv.FormatInt(int64(value/DefaultPriceMultiplier), 10)
	fraction := int64(value % DefaultPriceMultiplier)
	if fraction == 0 {
		return whole
	}
	if fraction < 0 {
		fraction = -fraction
		if whole == "0" {
			whole = "-0"
		}
	}
	fractionText := strconv.FormatInt(fraction, 10)
	return whole + "." + strings.TrimRight(strings.Repeat("0", 6-len(fractionText))+fractionText, "0")
}

// Valid reports whether a multiplier is within the permitted range.
func (value PriceMultiplier) Valid() bool { return value >= 0 && value <= 1_000_000_000 }

// MarshalJSON emits the exact multiplier as a string to avoid client float conversion.
func (value PriceMultiplier) MarshalJSON() ([]byte, error) {
	if !value.Valid() {
		return nil, errors.New("invalid price multiplier")
	}
	return json.Marshal(FormatPriceMultiplier(value))
}

// UnmarshalJSON accepts only an explicit decimal string; null does not mean the default value.
func (value *PriceMultiplier) UnmarshalJSON(data []byte) error {
	var text *string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == nil {
		return errors.New("price multiplier must be a decimal string")
	}
	parsed, err := ParsePriceMultiplier(*text)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// UnmarshalJSON requires both multiplier layers to be explicitly present; omitted fields cannot be interpreted as no multiplier.
func (multipliers *PriceMultipliers) UnmarshalJSON(data []byte) error {
	var fields struct {
		Group     *PriceMultiplier `json:"group"`
		AccessKey *PriceMultiplier `json:"access_key"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil {
		return err
	}
	if fields.Group == nil || fields.AccessKey == nil {
		return errors.New("group and access key price multipliers are required")
	}
	*multipliers = PriceMultipliers{Group: *fields.Group, AccessKey: *fields.AccessKey}
	return nil
}

// applyPriceMultipliers applies all multipliers once to the original nano-USD total without rounding between multipliers.
func applyPriceMultipliers(amount NanoUSD, multipliers PriceMultipliers) (NanoUSD, bool) {
	if amount < 0 || !multipliers.Group.Valid() || !multipliers.AccessKey.Valid() {
		return 0, false
	}
	numerator := big.NewInt(int64(amount))
	numerator.Mul(numerator, big.NewInt(int64(multipliers.Group)))
	numerator.Mul(numerator, big.NewInt(int64(multipliers.AccessKey)))
	denominator := big.NewInt(int64(DefaultPriceMultiplier) * int64(DefaultPriceMultiplier))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	remainder.Lsh(remainder, 1)
	if remainder.Cmp(denominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return NanoUSD(quotient.Int64()), true
}
