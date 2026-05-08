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
