package util

import (
	"regexp"
)

type Grep struct {
	regeps []*regexp.Regexp
}

func NewGrep(re1 *regexp.Regexp, regeps ...*regexp.Regexp) *Grep {
	return &Grep{regeps: append([]*regexp.Regexp{re1}, regeps...)}
}

func NewGrepFromStrings(re1 string, regeps ...string) (*Grep, error) {
	var r []*regexp.Regexp
	strs := append([]string{re1}, regeps...)
	for _, s := range strs {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		r = append(r, re)
	}
	return NewGrep(r[0], r[1:]...), nil
}

func (g *Grep) MatchAny(s string) bool {
	for _, re := range g.regeps {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func (g *Grep) MatchLinesInString(s string) (result []string) {
	lines := SplitLines(s)
	result = make([]string, 0, len(lines))
	for _, line := range lines {
		if g.MatchAny(line) {
			result = append(result, line)
		}
	}
	return
}

func (g *Grep) MatchWithTail(s string) (result []string) {
	lines := SplitLines(s)
	result = make([]string, 0, len(lines))
	for i, line := range lines {
		if g.MatchAny(line) {
			result = append(result, lines[i:]...)
			break
		}
	}
	return
}
