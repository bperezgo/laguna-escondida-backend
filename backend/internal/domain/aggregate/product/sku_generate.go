package product

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxSKUNameParts = 3

var areaSKUPrefixes = map[string]string{
	"kitchen":  "COC",
	"bar":      "BAR",
	"grill":    "PAR",
	"sushi":    "SUS",
	"desserts": "POS",
}

var skuFillerWords = map[string]bool{
	"A": true, "DE": true, "DEL": true, "CON": true, "LA": true, "EL": true,
	"LOS": true, "LAS": true, "Y": true, "EN": true, "AL": true,
}

// TakenSKUs is a case-insensitive set of SKUs a generated SKU must not collide with.
type TakenSKUs map[string]struct{}

func NewTakenSKUs(skus ...string) TakenSKUs {
	t := make(TakenSKUs, len(skus))
	for _, s := range skus {
		t.Add(s)
	}
	return t
}

func (t TakenSKUs) Add(sku string) {
	t[strings.ToUpper(sku)] = struct{}{}
}

func (t TakenSKUs) Has(sku string) bool {
	_, ok := t[strings.ToUpper(sku)]
	return ok
}

// GenerateSKU applies the rule in openspec specs/product-sku; frontend/lib/products/sku.ts must produce the same results.
func GenerateSKU(name string, productType ProductType, area string, taken TakenSKUs) string {
	parts := []string{skuPrefix(productType, area)}
	parts = append(parts, skuNameParts(name)...)
	base := strings.Join(parts, "-")

	if !taken.Has(base) {
		return base
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !taken.Has(candidate) {
			return candidate
		}
	}
}

func skuPrefix(productType ProductType, area string) string {
	area = strings.TrimSpace(area)
	if prefix, ok := areaSKUPrefixes[strings.ToLower(area)]; ok {
		return prefix
	}
	if folded := strings.Join(skuWords(area), ""); folded != "" {
		return truncate(folded, 3)
	}
	if productType == ProductTypeIngredient {
		return "ING"
	}
	return "PRD"
}

func skuNameParts(name string) []string {
	var parts []string
	for _, w := range skuWords(name) {
		if skuFillerWords[w] {
			continue
		}
		parts = append(parts, skuWordPart(w))
		if len(parts) == maxSKUNameParts {
			break
		}
	}
	return parts
}

func skuWordPart(word string) string {
	if word[0] >= '0' && word[0] <= '9' {
		end := strings.IndexFunc(word, func(r rune) bool { return r < '0' || r > '9' })
		if end == -1 {
			return word
		}
		return word[:end]
	}
	return truncate(word, 3)
}

// skuWords strips accents from s and returns its runs of ASCII letters and digits, uppercased.
func skuWords(s string) []string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	words := strings.FieldsFunc(b.String(), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
	for i, w := range words {
		words[i] = strings.ToUpper(w)
	}
	return words
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
