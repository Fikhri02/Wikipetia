// Package rank generates lexicographically ordered sort keys, so inserting or
// moving one item writes one row instead of renumbering its siblings.
package rank

import (
	"errors"
	"strings"
)

// Ordered by byte value, so plain string comparison matches this order.
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz"

// First is the key for the only item in an empty list.
func First() string {
	return string(alphabet[len(alphabet)/2])
}

// Between returns a key strictly between prev and next.
// prev == "" means "before every existing key"; next == "" means "after every
// existing key".
func Between(prev, next string) (string, error) {
	if prev != "" && next != "" && prev >= next {
		return "", errors.New("rank: prev must sort before next")
	}

	var out strings.Builder
	for i := 0; ; i++ {
		lo := 0
		if i < len(prev) {
			i0 := indexOf(prev[i])
			if i0 < 0 {
				return "", errors.New("rank: prev contains a character outside the alphabet")
			}
			lo = i0
		}

		hi := len(alphabet) // exclusive sentinel: "above every character"
		if next != "" {
			if i < len(next) {
				i1 := indexOf(next[i])
				if i1 < 0 {
					return "", errors.New("rank: next contains a character outside the alphabet")
				}
				hi = i1
			} else {
				return "", errors.New("rank: next is a prefix of prev")
			}
		}

		if hi-lo > 1 {
			out.WriteByte(alphabet[lo+(hi-lo)/2])
			return out.String(), nil
		}

		// lo and hi are adjacent (or equal): keep prev's character and descend.
		out.WriteByte(alphabet[lo])
		// Past this point any suffix sorts after prev and before next, so the
		// upper bound stops constraining us.
		if hi == lo+1 {
			next = ""
		}
	}
}

func indexOf(c byte) int {
	return strings.IndexByte(alphabet, c)
}
