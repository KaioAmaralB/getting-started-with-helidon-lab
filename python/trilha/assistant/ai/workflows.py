"""Composição dos fluxos de atendimento com estado por requisição."""
from trilha.assistant.guardrail.rules import answer_guardrail, GuardrailError, SupervisorOutputGuardrailError


class SupportWorkflows:
    def __init__(self, agents):
        self.agents = agents

    async def workflow(self, question, previous_summary):
        intent = await self.agents.classify(question, previous_summary)
        answer = await self.agents.expert(intent, question, previous_summary)
        summary = await self.agents.summarize(question, previous_summary, answer)
        return {"answer": answer, "summary": summary}

    async def mcp(self, question, previous_summary):
        answer = await self.agents.expert("ORDER", question, previous_summary, remote=True)
        summary = await self.agents.summarize(question, previous_summary, answer)
        return {"answer": answer, "summary": summary}

    async def supervisor(self, question, previous_summary):
        state = {"question": question, "previousSummary": previous_summary}
        answer = await self.agents.supervise(question, previous_summary, state)
        summary = await self.agents.summarize(state["question"], state["previousSummary"], answer)
        # A validação final sucede o resumo para manter a ordem do fluxo declarativo Java.
        try:
            answer_guardrail({"content": answer})
        except GuardrailError:
            raise SupervisorOutputGuardrailError() from None
        return {"answer": answer, "summary": summary}

    async def direct(self, question, previous_summary):
        answer = await self.agents.expert("RETURN", question, previous_summary)
        return {"answer": answer, "summary": ""}
