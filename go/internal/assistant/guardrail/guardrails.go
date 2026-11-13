package guardrail

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Failure struct{ Reason string }

func (f *Failure) Error() string { return f.Reason }

// JavaBlank matches Character.isWhitespace, including its non-breaking-space exclusions.
func JavaBlank(text string) bool {
	for _, r := range text {
		if r == '\u00a0' || r == '\u2007' || r == '\u202f' {
			return false
		}
		if !(unicode.Is(unicode.Zs, r) || r == '\u2028' || r == '\u2029' || (r >= 9 && r <= 13) || (r >= 0x1c && r <= 0x1f)) {
			return false
		}
	}
	return true
}

func Input(text string) error {
	lower := strings.ToLower(strings.ReplaceAll(text, "İ", "i\u0307"))
	if strings.Contains(lower, "-----begin private key-----") || strings.Contains(lower, "-----begin rsa private key-----") {
		return &Failure{"Entrada bloqueada pelo guardrail."}
	}
	return nil
}

// Output permits absent text only for intermediate tool-call messages.
func Output(text *string, hasTools bool, summary bool) error {
	if text == nil && hasTools {
		return nil
	}
	if text == nil || JavaBlank(*text) || utf8.RuneCountInString(*text) > 4000 {
		return &Failure{"Resposta vazia ou excessiva."}
	}
	lower := strings.ToLower(strings.ReplaceAll(*text, "İ", "i\u0307"))
	for _, forbidden := range []string{"estorno realizado", "troca aprovada", "-----begin"} {
		if strings.Contains(lower, forbidden) {
			return &Failure{"Resposta afirma uma operação indisponível ou contém material sensível."}
		}
	}
	if summary && utf8.RuneCountInString(*text) > 1500 {
		return &Failure{"Resumo excede 1500 pontos de código Unicode."}
	}
	return nil
}

func ValidateRequest(message, summary string) error {
	if JavaBlank(message) || utf8.RuneCountInString(message) > 2000 || utf8.RuneCountInString(summary) > 1500 {
		return fmt.Errorf("message deve conter 1 a 2000 pontos de código Unicode e summary até 1500.")
	}
	return Input(message + "\n" + summary)
}
