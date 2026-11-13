package rag

import (
	"encoding/json"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"os"
	"strings"
	"unicode"
)

// Tokenizer applies the BERT WordPiece pipeline configured in tokenizer.json.
// Truncation is loaded from that file (128 tokens for the bundled model).
type Tokenizer struct {
	Vocab   map[string]int64
	Inverse map[int64]string
	Maximum int
	Added   []string
	MaxWord int
}

func NewTokenizer(path string) (*Tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec struct {
		Model struct {
			Type    string           `json:"type"`
			Vocab   map[string]int64 `json:"vocab"`
			MaxWord int              `json:"max_input_chars_per_word"`
		} `json:"model"`
		Truncation *struct {
			Maximum int `json:"max_length"`
		} `json:"truncation"`
		Normalizer struct {
			Type      string `json:"type"`
			Lowercase bool   `json:"lowercase"`
			Clean     bool   `json:"clean_text"`
			Chinese   bool   `json:"handle_chinese_chars"`
		} `json:"normalizer"`
		Added []struct {
			Content string `json:"content"`
		} `json:"added_tokens"`
	}
	if err = json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	if spec.Model.Type != "WordPiece" || spec.Normalizer.Type != "BertNormalizer" || !spec.Normalizer.Lowercase || !spec.Normalizer.Clean || !spec.Normalizer.Chinese {
		return nil, fmt.Errorf("tokenizer BERT incompatível")
	}
	t := &Tokenizer{Vocab: spec.Model.Vocab, Inverse: map[int64]string{}, Maximum: 128, MaxWord: spec.Model.MaxWord}
	if spec.Truncation != nil && spec.Truncation.Maximum > 1 {
		t.Maximum = spec.Truncation.Maximum
	}
	for token, id := range t.Vocab {
		t.Inverse[id] = token
	}
	for _, token := range spec.Added {
		t.Added = append(t.Added, token.Content)
	}
	return t, nil
}

func chinese(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3400 && r <= 0x4dbf) || (r >= 0x20000 && r <= 0x2a6df) || (r >= 0x2a700 && r <= 0x2b73f) || (r >= 0x2b740 && r <= 0x2b81f) || (r >= 0x2b820 && r <= 0x2ceaf) || (r >= 0xf900 && r <= 0xfaff) || (r >= 0x2f800 && r <= 0x2fa1f)
}
func punctuation(r rune) bool {
	return (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) || unicode.IsPunct(r)
}

func normalize(text string) string {
	var cleaned strings.Builder
	for _, r := range text {
		if r == 0 || r == 0xfffd || ((unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r)) && r != '\t' && r != '\n' && r != '\r') {
			continue
		}
		if unicode.IsSpace(r) {
			cleaned.WriteByte(' ')
			continue
		}
		if chinese(r) {
			cleaned.WriteByte(' ')
			cleaned.WriteRune(r)
			cleaned.WriteByte(' ')
		} else {
			cleaned.WriteRune(r)
		}
	}
	// strip_accents=null inherits lowercase=true in BertNormalizer.
	var stripped strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(cleaned.String())) {
		if !unicode.Is(unicode.Mn, r) {
			stripped.WriteRune(r)
		}
	}
	return stripped.String()
}

func (t *Tokenizer) wordPieces(text string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range normalize(text) {
		if unicode.IsSpace(r) {
			flush()
		} else if punctuation(r) {
			flush()
			words = append(words, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	flush()
	var result []string
	for _, word := range words {
		runes := []rune(word)
		if len(runes) > t.MaxWord {
			result = append(result, "[UNK]")
			continue
		}
		pieces := []string{}
		start := 0
		bad := false
		for start < len(runes) {
			end := len(runes)
			found := ""
			for end > start {
				candidate := string(runes[start:end])
				if start > 0 {
					candidate = "##" + candidate
				}
				if _, ok := t.Vocab[candidate]; ok {
					found = candidate
					break
				}
				end--
			}
			if found == "" {
				bad = true
				break
			}
			pieces = append(pieces, found)
			start = end
		}
		if bad {
			result = append(result, "[UNK]")
		} else {
			result = append(result, pieces...)
		}
	}
	return result
}

func (t *Tokenizer) Tokens(text string) []string {
	content := []string{}
	for len(text) > 0 {
		next := -1
		special := ""
		for _, token := range t.Added {
			if p := strings.Index(text, token); p >= 0 && (next < 0 || p < next) {
				next = p
				special = token
			}
		}
		if next < 0 {
			content = append(content, t.wordPieces(text)...)
			break
		}
		content = append(content, t.wordPieces(text[:next])...)
		content = append(content, special)
		text = text[next+len(special):]
	}
	if len(content) > t.Maximum-2 {
		content = content[:t.Maximum-2]
	}
	return append(append([]string{"[CLS]"}, content...), "[SEP]")
}
func (t *Tokenizer) IDs(tokens []string) []int64 {
	ids := make([]int64, len(tokens))
	for i, token := range tokens {
		id, ok := t.Vocab[token]
		if !ok {
			id = t.Vocab["[UNK]"]
		}
		ids[i] = id
	}
	return ids
}

// buildSentence reproduces DJL 0.36.0 token joining without Hugging Face decoder cleanup.
func (t *Tokenizer) buildSentence(tokens []string) string {
	text := strings.ReplaceAll(strings.Join(tokens, " "), " ##", "")
	return strings.TrimFunc(text, func(r rune) bool { return r <= 0x20 })
}
func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func (t *Tokenizer) EncodePartition(tokens []string) []int64 {
	text := t.buildSentence(tokens)
	rebuilt := t.Tokens(text)
	if len(rebuilt) < 2 || !equalTokens(rebuilt[1:len(rebuilt)-1], tokens) {
		text = strings.Join(tokens, "")
	}
	return t.IDs(t.Tokens(text))
}
func partition(tokens []string) [][]string {
	var parts [][]string
	for from := 1; from < len(tokens)-1; {
		to := from + 510
		if to >= len(tokens)-1 {
			to = len(tokens) - 1
		} else {
			for to > from && strings.HasPrefix(tokens[to], "##") {
				to--
			}
			if to == from {
				to = from + 510
			}
		}
		parts = append(parts, tokens[from:to])
		from = to
	}
	return parts
}
