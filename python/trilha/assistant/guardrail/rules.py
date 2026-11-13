"""Validações de entrada, resposta e resumo."""
class GuardrailError(Exception):
    pass


class SupervisorOutputGuardrailError(GuardrailError):
    pass


class InvalidRequest(Exception):
    pass


# Reproduz Character.isWhitespace, que exclui U+00A0, U+2007 e U+202F.
def java_is_blank(value):
    def whitespace(code):
        return (9 <= code <= 13 or 0x1C <= code <= 0x20 or code == 0x1680
                or 0x2000 <= code <= 0x2006 or 0x2008 <= code <= 0x200A
                or code in (0x2028, 0x2029, 0x205F, 0x3000))
    return all(whitespace(ord(char)) for char in value)


def input_guardrail(text):
    lower = text.lower()
    if "-----begin private key-----" in lower or "-----begin rsa private key-----" in lower:
        raise GuardrailError("Entrada bloqueada pelo guardrail.")


def answer_guardrail(message):
    text = message.get("content")
    if text is None and message.get("tool_calls"):
        return
    if not isinstance(text, str) or java_is_blank(text) or len(text) > 4000:
        raise GuardrailError("Resposta vazia ou excessiva.")
    lower = text.lower()
    if any(marker in lower for marker in ("estorno realizado", "troca aprovada", "-----begin")):
        raise GuardrailError("Resposta bloqueada pelo guardrail.")


def summary_guardrail(message):
    answer_guardrail(message)
    if isinstance(message.get("content"), str) and len(message["content"]) > 1500:
        raise GuardrailError("Resumo excede 1500 pontos de código Unicode.")


def validate_request(body):
    if not isinstance(body, dict):
        raise InvalidRequest()
    message, summary = body.get("message"), body.get("summary")
    if (not isinstance(message, str) or java_is_blank(message) or len(message) > 2000
            or (summary is not None and (not isinstance(summary, str) or len(summary) > 1500))):
        raise InvalidRequest()
    summary = "" if summary is None else summary
    input_guardrail(message + "\n" + summary)
    return message, summary
