package internal

// sliceContains returns true if needle is in haystack.
func SliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
