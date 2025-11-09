package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func NumberToWords(num string) string {
	integerPart, err := strconv.Atoi(num)
	if err != nil {
		return ""
	}

	if integerPart == 0 {
		return "cero pesos"
	}

	ones := []string{"", "uno", "dos", "tres", "cuatro", "cinco", "seis", "siete", "ocho", "nueve"}
	tens := []string{"", "diez", "veinte", "treinta", "cuarenta", "cincuenta", "sesenta", "setenta", "ochenta", "noventa"}
	special := map[int]string{
		11: "once", 12: "doce", 13: "trece", 14: "catorce", 15: "quince",
		16: "dieciséis", 17: "diecisiete", 18: "dieciocho", 19: "diecinueve",
		21: "veintiuno", 22: "veintidós", 23: "veintitrés", 24: "veinticuatro",
		25: "veinticinco", 26: "veintiséis", 27: "veintisiete", 28: "veintiocho", 29: "veintinueve",
	}

	if word, ok := special[integerPart]; ok {
		return word + " pesos"
	}

	if integerPart < 10 {
		return ones[integerPart] + " pesos"
	}

	if integerPart < 100 {
		tensDigit := integerPart / 10
		onesDigit := integerPart % 10
		if onesDigit == 0 {
			return tens[tensDigit] + " pesos"
		}
		return tens[tensDigit] + " y " + ones[onesDigit] + " pesos"
	}

	if integerPart < 1000 {
		hundredsDigit := integerPart / 100
		remainder := integerPart % 100
		hundredsWord := "cien"
		if hundredsDigit == 1 && remainder > 0 {
			hundredsWord = "ciento"
		} else if hundredsDigit > 1 {
			hundredsWord = ones[hundredsDigit] + "cientos"
		}
		if remainder == 0 {
			return hundredsWord + " pesos"
		}
		remainderStr := NumberToWords(strconv.Itoa(remainder))
		remainderStr = strings.TrimSuffix(remainderStr, " pesos")
		return hundredsWord + " " + remainderStr + " pesos"
	}

	if integerPart < 1000000 {
		thousands := integerPart / 1000
		remainder := integerPart % 1000
		thousandsWord := "mil"
		if thousands > 1 {
			thousandsStr := NumberToWords(strconv.Itoa(thousands))
			thousandsStr = strings.TrimSuffix(thousandsStr, " pesos")
			thousandsWord = thousandsStr + " mil"
		}
		if remainder == 0 {
			return thousandsWord + " pesos"
		}
		remainderStr := NumberToWords(strconv.Itoa(remainder))
		remainderStr = strings.TrimSuffix(remainderStr, " pesos")
		return thousandsWord + " " + remainderStr + " pesos"
	}

	return fmt.Sprintf("%d pesos", integerPart)
}
