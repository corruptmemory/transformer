package grubutils

import (
	"bytes"
	"strings"
)

// menuentry 'Ubuntu' --class ubuntu --class gnu-linux --class gnu --class os $menuentry_id_option 'gnulinux-simple-90b2d032-13fc-4dc6-a239-4972c9678e88'
type MenuEntry struct {
	Name  string
	Class []string
	Id    string
	Body  string
}

// submenu 'Advanced options for Ubuntu' $menuentry_id_option 'gnulinux-advanced-90b2d032-13fc-4dc6-a239-4972c9678e88'
type SubMenu struct {
	Name    string
	Id      string
	Entries []MenuEntry
}

const (
	menuentry         = "menuentry"
	submenu           = "submenu"
	bodyStart         = "{"
	bodyEnd           = "}"
	menuEntryIDOption = "$menuentry_id_option"
	bareID            = "--id"
	class             = "--class"
)

var whiteSpace = []byte(" \t")

type grubTokenTag int

const (
	gtError grubTokenTag = iota
	gtNone
	gtIgnore
	gtMenuEntry
	gtSubMenu
	gtBodyStart
	gtBodyEnd
	gtClass
	gtId
	gtBodyEntry
)

type grubToken struct {
	tag grubTokenTag
	val lexeme
	err error
}

type lexeme struct {
	val        string
	quoted     bool
	quote      byte
	byteOffset int
	line       int
	err        error
}

func (l *lexeme) lcVal() string {
	return strings.ToLower(l.val)
}

type grubStringLexer struct {
	out    chan<- lexeme
	data   []byte
	offset int
	line   int
}

func (g *grubStringLexer) nextByte() (b byte, ok bool) {
	if g.offset >= len(g.data) {
		return 0, false
	}
	b = g.data[g.offset]
	g.offset++
	return b, true
}

type grubStringLexerFunc func(out chan<- lexeme) (next grubStringLexerFunc)

func (g *grubStringLexer) initStringLexerState(out chan<- lexeme) (next grubStringLexerFunc) {
	buf := &bytes.Buffer{}
	maybeSend := func() {
		if buf.Len() > 0 {
			out <- lexeme{val: buf.String(), byteOffset: g.offset - buf.Len(), line: g.line}
			buf.Reset()
		}
	}
	for b, ok := g.nextByte(); ok; b, ok = g.nextByte() {
		switch {
		case b == '\n':
			maybeSend()
			g.line++
		case b == '"' || b == '\'':
			return g.quotedLexerState(b, g.initStringLexerState)
		case bytes.IndexByte(whiteSpace, b) >= 0:
			maybeSend()
			continue
		default:
			buf.Write([]byte{b})
		}
	}
	maybeSend()
	return nil
}

func expandEscapedChar(in byte) byte {
	switch in {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'v':
		return '\v'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	default:
		return in
	}
}

func (g *grubStringLexer) quotedLexerState(quoteChar byte, andThen grubStringLexerFunc) func(out chan<- lexeme) (next grubStringLexerFunc) {
	return func(out chan<- lexeme) (next grubStringLexerFunc) {
		buf := &bytes.Buffer{}
		send := func() {
			out <- lexeme{val: buf.String(), byteOffset: g.offset - buf.Len(), line: g.line, quoted: true, quote: quoteChar}
		}
		var handleEscaped bool
		for b, ok := g.nextByte(); ok; b, ok = g.nextByte() {
			switch {
			case !handleEscaped && b == '\\':
				handleEscaped = true
			case handleEscaped:
				buf.Write([]byte{expandEscapedChar(b)})
				handleEscaped = false
			case b == quoteChar:
				send()
				return andThen
			default:
				buf.Write([]byte{b})
			}
		}
		send()
		return nil
	}
}

type grubLexerFunc func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc)

func initGrubLexerState(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	for l := range in {
		if l.err != nil {
			out <- grubToken{tag: gtError, val: l, err: l.err}
			return nil
		}
		switch l.lcVal() {
		case menuentry:
			out <- grubToken{tag: gtNone}
			return menuEntryState(initGrubLexerState)
		case submenu:
			out <- grubToken{tag: gtNone}
			return subMenuState(initGrubLexerState)
		default:
			out <- grubToken{tag: gtIgnore, val: l}
			return initGrubLexerState
		}
	}
	return nil
}

func menuEntryState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			switch l.lcVal() {
			case class:
				out <- grubToken{tag: gtNone}
				return classState(menuEntryState(andThen))
			case menuEntryIDOption, bareID:
				out <- grubToken{tag: gtNone}
				return idState(menuEntryState(andThen))
			case bodyStart:
				out <- grubToken{tag: gtBodyStart, val: l}
				return bodyState(andThen)
			default:
				out <- grubToken{tag: gtMenuEntry, val: l}
				return menuEntryState(andThen)
			}
		}
		return nil
	}
}

func classState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			out <- grubToken{tag: gtClass, val: l}
			return andThen
		}
		return nil
	}
}

func idState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			out <- grubToken{tag: gtId, val: l}
			return andThen
		}
		return nil
	}
}

func subMenuState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			switch l.lcVal() {
			case menuEntryIDOption, bareID:
				out <- grubToken{tag: gtNone}
				return idState(subMenuState(andThen))
			case bodyStart:
				out <- grubToken{tag: gtBodyStart, val: l}
				return subMenuBodyState(andThen)
			default:
				out <- grubToken{tag: gtSubMenu, val: l}
				return subMenuState(andThen)
			}

		}
		return nil
	}
}

func subMenuBodyState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			switch l.val {
			case menuentry:
				out <- grubToken{tag: gtNone}
				return menuEntryState(subMenuBodyState(andThen))
			case bodyStart:
				out <- grubToken{tag: gtBodyEntry, val: l}
				if l.quoted {
					return bodyState(andThen)
				}
				return bodyState(andThen)
			case bodyEnd:
				out <- grubToken{tag: gtBodyEnd, val: l}
				return andThen
			default:
				out <- grubToken{tag: gtBodyEntry, val: l}
				return bodyState(andThen)
			}

		}
		return nil
	}
}

func bodyState(andThen grubLexerFunc) func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
	return func(out chan<- grubToken, in <-chan lexeme) (next grubLexerFunc) {
		for l := range in {
			if l.err != nil {
				out <- grubToken{tag: gtError, val: l, err: l.err}
				return nil
			}
			switch l.val {
			case bodyStart:
				out <- grubToken{tag: gtBodyEntry, val: l}
				return bodyState(andThen)
			case bodyEnd:
				out <- grubToken{tag: gtBodyEnd, val: l}
				return andThen
			default:
				out <- grubToken{tag: gtBodyEntry, val: l}
				return bodyState(andThen)
			}

		}
		return nil
	}
}

func grubLexer(file []byte) (out <-chan grubToken) {
	outCh := make(chan grubToken, 1)
	lexemeCh := make(chan lexeme, 1)
	go func() {
		defer close(lexemeCh)
		sl := &grubStringLexer{
			out:  lexemeCh,
			data: file,
		}
		for next := sl.initStringLexerState(lexemeCh); next != nil; next = next(lexemeCh) {
		}
	}()
	go func() {
		defer close(outCh)
		for next := initGrubLexerState(outCh, lexemeCh); next != nil; next = next(outCh, lexemeCh) {
		}
	}()
	return outCh
}

func buildMenuEntry(name string, tokenStream <-chan grubToken) (menuEntry MenuEntry, err error) {
	bodyBuffer := &bytes.Buffer{}
	var bodyLine int
	menuEntry.Name = name
	var lineValues []string
	for token := range tokenStream {
		switch token.tag {
		case gtClass:
			menuEntry.Class = append(menuEntry.Class, token.val.val)
		case gtId:
			menuEntry.Id = token.val.val
		case gtBodyStart:
			bodyLine = token.val.line
		case gtBodyEnd:
			if len(lineValues) > 0 {
				bodyBuffer.WriteString(strings.Join(lineValues, " "))
			}
			menuEntry.Body = strings.TrimSpace(bodyBuffer.String())
			return
		case gtBodyEntry:
			if token.val.line != bodyLine {
				bodyLine = token.val.line
				if len(lineValues) > 0 {
					bodyBuffer.WriteString(strings.Join(lineValues, " "))
					lineValues = lineValues[:0]
				}
				bodyBuffer.WriteByte('\n')
			}
			var toAppend string
			if token.val.quoted {
				toAppend = string(token.val.quote) + token.val.val + string(token.val.quote)
			} else {
				toAppend = token.val.val
			}
			lineValues = append(lineValues, toAppend)
		case gtError:
			return MenuEntry{}, token.err
		default:
			continue
		}
	}
	return
}

func buildSubMenuEntry(name string, tokenStream <-chan grubToken) (subMenu SubMenu, err error) {
	subMenu.Name = name
	for token := range tokenStream {
		switch token.tag {
		case gtId:
			subMenu.Id = token.val.val
		case gtMenuEntry:
			menuEntry, err := buildMenuEntry(token.val.val, tokenStream)
			if err != nil {
				return SubMenu{}, err
			}
			subMenu.Entries = append(subMenu.Entries, menuEntry)
		case gtError:
			return SubMenu{}, token.err
		case gtBodyEnd:
			return
		default:
			continue
		}
	}
	return
}

func GrubParser(fileString string) (menus []MenuEntry, subMenus []SubMenu, err error) {
	file := []byte(fileString)
	tokenStream := grubLexer(file)
	for token := range tokenStream {
		switch token.tag {
		case gtMenuEntry:
			menuEntry, err := buildMenuEntry(token.val.val, tokenStream)
			if err != nil {
				return nil, nil, err
			}
			menus = append(menus, menuEntry)
		case gtSubMenu:
			subMenu, err := buildSubMenuEntry(token.val.val, tokenStream)
			if err != nil {
				return nil, nil, err
			}
			subMenus = append(subMenus, subMenu)
		case gtError:
			return nil, nil, token.err
		default:
			continue
		}
	}
	return
}
