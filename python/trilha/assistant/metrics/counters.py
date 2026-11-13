"""Contadores de chamadas ao modelo, tokens e ferramentas."""
from threading import Lock


class ModelMetrics:
    FIELDS = ("modelRequests", "modelResponses", "modelErrors", "inputTokens", "outputTokens",
              "responsesWithoutCompleteUsage", "requestedToolCalls")

    def __init__(self):
        self._values = dict.fromkeys(self.FIELDS, 0)
        self._lock = Lock()

    def requested(self):
        with self._lock:
            self._values["modelRequests"] += 1

    def failed(self):
        with self._lock:
            self._values["modelErrors"] += 1

    def responded(self, result):
        usage = result.get("usage") or {}
        incoming, outgoing = usage.get("prompt_tokens"), usage.get("completion_tokens")
        message = result["choices"][0]["message"]
        with self._lock:
            self._values["modelResponses"] += 1
            if incoming is None or outgoing is None:
                self._values["responsesWithoutCompleteUsage"] += 1
            if incoming is not None:
                self._values["inputTokens"] += incoming
            if outgoing is not None:
                self._values["outputTokens"] += outgoing
            self._values["requestedToolCalls"] += len(message.get("tool_calls") or [])

    def snapshot(self):
        with self._lock:
            return self._values.copy()
