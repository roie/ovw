package tui

func textCursor(value string, cursor int) int {
	length := len([]rune(value))
	if cursor < 0 {
		return 0
	}
	if cursor > length {
		return length
	}
	return cursor
}

func textMoveLeft(value string, cursor int) int {
	cursor = textCursor(value, cursor)
	if cursor == 0 {
		return 0
	}
	return cursor - 1
}

func textMoveRight(value string, cursor int) int {
	cursor = textCursor(value, cursor)
	if cursor >= len([]rune(value)) {
		return cursor
	}
	return cursor + 1
}

func textMoveStart(value string, cursor int) int {
	return 0
}

func textMoveEnd(value string, cursor int) int {
	return len([]rune(value))
}

func textInsert(value string, cursor int, text string) (string, int) {
	if text == "" {
		return value, textCursor(value, cursor)
	}
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	inserted := []rune(text)
	next := make([]rune, 0, len(runes)+len(inserted))
	next = append(next, runes[:cursor]...)
	next = append(next, inserted...)
	next = append(next, runes[cursor:]...)
	return string(next), cursor + len(inserted)
}

func textClearBefore(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	return string(runes[cursor:]), 0
}

func textClearAfter(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	return string(runes[:cursor]), cursor
}

func textDeletePreviousWord(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	if cursor == 0 {
		return value, cursor
	}
	start := cursor
	for start > 0 && runes[start-1] == ' ' {
		start--
	}
	for start > 0 && runes[start-1] != ' ' {
		start--
	}
	next := make([]rune, 0, len(runes)-(cursor-start))
	next = append(next, runes[:start]...)
	next = append(next, runes[cursor:]...)
	return string(next), start
}

func textBackspace(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	if cursor == 0 {
		return value, cursor
	}
	next := make([]rune, 0, len(runes)-1)
	next = append(next, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	return string(next), cursor - 1
}

func textDelete(value string, cursor int) (string, int) {
	runes := []rune(value)
	cursor = textCursor(value, cursor)
	if cursor >= len(runes) {
		return value, cursor
	}
	next := make([]rune, 0, len(runes)-1)
	next = append(next, runes[:cursor]...)
	next = append(next, runes[cursor+1:]...)
	return string(next), cursor
}
