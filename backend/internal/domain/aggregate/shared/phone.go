package shared

import (
	"strings"

	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/nyaruka/phonenumbers"
)

type PhoneErrorCode string

const (
	CodeInvalidPhone PhoneErrorCode = "INVALID_PHONE"
)

func NewInvalidPhoneError(raw string) *baseError.BaseError {
	return baseError.NewBaseErrorWithField(
		baseError.ErrorCode(CodeInvalidPhone),
		"phone is not a valid number",
		raw,
	)
}

// Phone is a customer phone in E.164 form, valid for its country.
type Phone struct {
	value string
}

// NewPhone reads typed input: "+..." as international, anything else against homeRegion
// (ISO 3166 alpha-2), which also accepts that region's numbers written with the country code.
func NewPhone(raw string, homeRegion string) (*Phone, error) {
	cleaned := stripPhoneSeparators(raw)
	if cleaned == "" {
		return nil, NewInvalidPhoneError(raw)
	}
	region := strings.ToUpper(homeRegion)
	if strings.HasPrefix(cleaned, "+") {
		region = ""
	}
	return parsePhone(raw, cleaned, region)
}

// NewInternationalPhone reads a messaging-channel sender id, which always carries the country
// code without "+"; reading it against a home region could mistake it for a national number.
func NewInternationalPhone(digits string) (*Phone, error) {
	cleaned := strings.TrimPrefix(stripPhoneSeparators(digits), "+")
	if cleaned == "" {
		return nil, NewInvalidPhoneError(digits)
	}
	return parsePhone(digits, "+"+cleaned, "")
}

func parsePhone(raw, cleaned, region string) (*Phone, error) {
	number, err := phonenumbers.Parse(cleaned, region)
	if err != nil || !phonenumbers.IsValidNumber(number) {
		return nil, NewInvalidPhoneError(raw)
	}
	return &Phone{value: phonenumbers.Format(number, phonenumbers.E164)}, nil
}

func stripPhoneSeparators(raw string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '(', ')', '\t':
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
}

func (p *Phone) Value() string {
	return p.value
}

func (p *Phone) String() string {
	return p.value
}
