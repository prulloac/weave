package search

import (
	"math"
	"strings"
)

// RoundScore rounds a relevance score to four decimals.
func RoundScore(score float64) float64 {
	return math.Round(score*10000) / 10000
}

// TokenSimilarity returns the normalized similarity used by fuzzy search.
func TokenSimilarity(a, b string) float64 {
	return tokenSimilarity(a, b)
}

func tokenSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	if len(a) < 3 || len(b) < 3 {
		if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
			return 0.75
		}
		return 0
	}
	if isASCII(a) && isASCII(b) {
		return math.Max(normalizedLevenshteinBytes([]byte(a), []byte(b)), trigramDiceBytes([]byte(a), []byte(b)))
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

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 {
			return false
		}
	}
	return true
}

func normalizedLevenshteinBytes(a, b []byte) float64 {
	length := len(a)
	if len(b) > length {
		length = len(b)
	}
	if length == 0 {
		return 1
	}
	return 1 - float64(levenshteinBytes(a, b))/float64(length)
}

func levenshteinBytes(a, b []byte) int {
	if len(b) > 64 {
		return levenshtein([]rune(string(a)), []rune(string(b)))
	}
	var previous, current [65]int
	for index := 0; index <= len(b); index++ {
		previous[index] = index
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

func trigramDiceBytes(a, b []byte) float64 {
	if len(a) > 62 || len(b) > 62 {
		return trigramDice([]rune(string(a)), []rune(string(b)))
	}
	var aSet, bSet [64]uint32
	aCount := fillTrigramsBytes(a, &aSet)
	bCount := fillTrigramsBytes(b, &bSet)
	if aCount == 0 || bCount == 0 {
		return 0
	}
	intersection := 0
	for i := 0; i < aCount; i++ {
		for j := 0; j < bCount; j++ {
			if aSet[i] == bSet[j] {
				intersection++
				break
			}
		}
	}
	return 2 * float64(intersection) / float64(aCount+bCount)
}

func fillTrigramsBytes(token []byte, target *[64]uint32) int {
	count := 0
	write := func(a, b, c byte) {
		target[count] = uint32(a)<<16 | uint32(b)<<8 | uint32(c)
		count++
	}
	write('#', token[0], token[1])
	for index := 0; index+2 < len(token); index++ {
		write(token[index], token[index+1], token[index+2])
	}
	write(token[len(token)-2], token[len(token)-1], '#')
	return count
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
