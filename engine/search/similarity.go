package search

import (
	"math"
	"strings"
)

// RoundScore rounds a relevance score to four decimals.
func RoundScore(score float64) float64 {
	return math.Round(score*10000) / 10000
}

func tokenSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	aRunes := []rune(a)
	bRunes := []rune(b)
	if len(aRunes) < 3 || len(bRunes) < 3 {
		if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
			return 0.75
		}
		return 0
	}
	return math.Max(normalizedLevenshtein(aRunes, bRunes), trigramDice(aRunes, bRunes))
}

func normalizedLevenshtein(a, b []rune) float64 {
	length := len(a)
	if len(b) > length {
		length = len(b)
	}
	if length == 0 {
		return 1
	}
	return 1 - float64(levenshtein(a, b))/float64(length)
}

func levenshtein(a, b []rune) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(
				current[j-1]+1,
				previous[j]+1,
				previous[j-1]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

func trigramDice(a, b []rune) float64 {
	aSet := runeTrigrams(a)
	bSet := runeTrigrams(b)
	if len(aSet) == 0 || len(bSet) == 0 {
		return 0
	}
	intersection := 0
	for trigram := range aSet {
		if bSet[trigram] {
			intersection++
		}
	}
	return 2 * float64(intersection) / float64(len(aSet)+len(bSet))
}

func runeTrigrams(runes []rune) map[string]bool {
	padded := append([]rune{'#'}, runes...)
	padded = append(padded, '#')
	result := make(map[string]bool)
	for i := 0; i+3 <= len(padded); i++ {
		result[string(padded[i:i+3])] = true
	}
	return result
}
