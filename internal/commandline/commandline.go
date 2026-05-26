package commandline

import (
	"errors"
	"strings"
	"unicode"
)

func Split(value string) ([]string, error) {
	runes := []rune(value)
	args := []string{}
	var current strings.Builder
	started := false
	inSingle := false
	inDouble := false

	flush := func() {
		args = append(args, current.String())
		current.Reset()
		started = false
	}

	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if inSingle {
			if r == '\'' {
				inSingle = false
				continue
			}
			current.WriteRune(r)
			started = true
			continue
		}
		if inDouble {
			if r == '"' {
				inDouble = false
				continue
			}
			if r == '\\' && index+1 < len(runes) && (runes[index+1] == '"' || runes[index+1] == '\\') {
				index++
				current.WriteRune(runes[index])
				started = true
				continue
			}
			current.WriteRune(r)
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				flush()
			}
			continue
		}
		switch r {
		case '\'':
			inSingle = true
			started = true
		case '"':
			inDouble = true
			started = true
		case '\\':
			if index+1 < len(runes) && escapableOutsideQuotes(runes[index+1]) {
				index++
				current.WriteRune(runes[index])
			} else {
				current.WriteRune(r)
			}
			started = true
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if inSingle || inDouble {
		return nil, errors.New("unterminated quoted string")
	}
	if started {
		flush()
	}
	return args, nil
}

func escapableOutsideQuotes(r rune) bool {
	return unicode.IsSpace(r) || r == '\'' || r == '"' || r == '\\'
}
