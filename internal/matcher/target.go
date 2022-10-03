package matcher

type Target interface {
	Result() any
	MatchStrings() []Keyword
}

type StandardTarget struct {
	result       any
	matchStrings []Keyword
}

func (s *StandardTarget) Result() any             { return s.result }
func (s *StandardTarget) MatchStrings() []Keyword { return s.matchStrings }

type StringTarget struct {
	str string
}

func (s *StringTarget) Result() any { return s.str }
func (s *StringTarget) MatchStrings() []Keyword {
	return []Keyword{{String: s.str, Weight: 1.0}}
}
