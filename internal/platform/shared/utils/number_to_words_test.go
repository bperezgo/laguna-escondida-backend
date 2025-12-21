package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNumberToWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "125500",
			input:    "125500",
			expected: "ciento veinticinco mil quinientos pesos",
		},
		{
			name:     "1345678",
			input:    "1345678",
			expected: "un millón trescientos cuarenta y cinco mil seiscientos setenta y ocho pesos",
		},
		{
			name:     "13589.67 - decimals should be ignored",
			input:    "13589.67",
			expected: "trece mil quinientos ochenta y nueve pesos",
		},
		{
			name:     "zero",
			input:    "0",
			expected: "cero pesos",
		},
		{
			name:     "single digit",
			input:    "5",
			expected: "cinco pesos",
		},
		{
			name:     "special number 15",
			input:    "15",
			expected: "quince pesos",
		},
		{
			name:     "special number 21",
			input:    "21",
			expected: "veintiuno pesos",
		},
		{
			name:     "one hundred",
			input:    "100",
			expected: "ciento pesos",
		},
		{
			name:     "one thousand",
			input:    "1000",
			expected: "mil pesos",
		},
		{
			name:     "invalid input",
			input:    "invalid",
			expected: "",
		},
		// Additional 20 scenarios
		{
			name:     "eleven - special teen",
			input:    "11",
			expected: "once pesos",
		},
		{
			name:     "sixteen - special teen with accent",
			input:    "16",
			expected: "dieciséis pesos",
		},
		{
			name:     "twenty-two - special twenties",
			input:    "22",
			expected: "veintidós pesos",
		},
		{
			name:     "thirty-three - compound tens",
			input:    "33",
			expected: "treinta y tres pesos",
		},
		{
			name:     "fifty",
			input:    "50",
			expected: "cincuenta pesos",
		},
		{
			name:     "ninety-nine",
			input:    "99",
			expected: "noventa y nueve pesos",
		},
		{
			name:     "two hundred",
			input:    "200",
			expected: "doscientos pesos",
		},
		{
			name:     "three hundred and one",
			input:    "301",
			expected: "trescientos uno pesos",
		},
		{
			name:     "five hundred",
			input:    "500",
			expected: "quinientos pesos",
		},
		{
			name:     "seven hundred seventy-seven",
			input:    "777",
			expected: "setecientos setenta y siete pesos",
		},
		{
			name:     "nine hundred",
			input:    "900",
			expected: "novecientos pesos",
		},
		{
			name:     "two thousand",
			input:    "2000",
			expected: "dos mil pesos",
		},
		{
			name:     "ten thousand",
			input:    "10000",
			expected: "diez mil pesos",
		},
		{
			name:     "fifty thousand",
			input:    "50000",
			expected: "cincuenta mil pesos",
		},
		{
			name:     "ninety-nine thousand nine hundred ninety-nine",
			input:    "99999",
			expected: "noventa y nueve mil novecientos noventa y nueve pesos",
		},
		{
			name:     "one million",
			input:    "1000000",
			expected: "un millón pesos",
		},
		{
			name:     "two million",
			input:    "2000000",
			expected: "dos millones pesos",
		},
		{
			name:     "five million five hundred",
			input:    "5000500",
			expected: "cinco millones quinientos pesos",
		},
		{
			name:     "ten million",
			input:    "10000000",
			expected: "diez millones pesos",
		},
		{
			name:     "large decimal - should ignore cents",
			input:    "999999.99",
			expected: "novecientos noventa y nueve mil novecientos noventa y nueve pesos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NumberToWords(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
