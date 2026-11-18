package utils

import (
	"strconv"

	ntw "moul.io/number-to-words"
)

func NumberToWords(num string) string {
	floatPart, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return ""
	}

	integerPart := int(floatPart)

	if integerPart == 0 {
		return "cero pesos"
	}

	return ntw.IntegerToEsEs(integerPart) + " pesos"
}
