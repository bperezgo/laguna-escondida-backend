package buildinfo

import (
	"strconv"
	"strings"
)

// ParseVersion reads "vMAJOR.MINOR.PATCH" (the "v" and any "-suffix" are optional/ignored).
// ok is false for anything else, such as the unstamped "dev".
func ParseVersion(s string) (v [3]int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	nums := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		nums = append(nums, n)
	}
	copy(v[:], nums)
	return v, true
}

// VersionLess reports whether a is an older release than b. It is false when either
// is not a parsable version.
func VersionLess(a, b string) bool {
	av, aok := ParseVersion(a)
	bv, bok := ParseVersion(b)
	if !aok || !bok {
		return false
	}
	return av[0] < bv[0] || av[0] == bv[0] && (av[1] < bv[1] || av[1] == bv[1] && av[2] < bv[2])
}
