"""Agentes de classificação, atendimento, resumo e supervisão."""
import json

from trilha.assistant.ai.client import ProviderError
from trilha.assistant.guardrail.rules import input_guardrail, answer_guardrail, summary_guardrail
from trilha.domain.catalog import query_order


def assistant_message(message):
    # Preserva arguments no histórico sem reserializar o JSON recebido do modelo.
    result = {"role": "assistant"}
    if message.get("content") is not None:
        result["content"] = message["content"]
    if message.get("tool_calls"):
        result["tool_calls"] = message["tool_calls"]
    return result


def java_string(value):
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, dict):
        return "{" + ", ".join(java_string(key) + "=" + java_string(item) for key, item in value.items()) + "}"
    if isinstance(value, list):
        return "[" + ", ".join(java_string(item) for item in value) + "]"
    return str(value)


def parse_intent(text):
    trim_chars = "".join(map(chr, range(33)))
    def candidate(value):
        value = value.strip(trim_chars)
        if value.startswith("[") and value.endswith("]"):
            value = value[1:-1].strip(trim_chars)
        upper = value.upper()
        return upper if upper in ("ORDER", "RETURN") else None
    result = candidate(text)
    if result:
        return result
    try:
        parsed = json.loads(text)
        value = parsed.get("value") if isinstance(parsed, dict) else parsed
        if isinstance(value, str) and candidate(value):
            return candidate(value)
    except (ValueError, TypeError):
        pass
    raise ProviderError("InvalidIntent")


def parse_plan(text):
    if not isinstance(text, str):
        raise ProviderError("InvalidPlannerResponse")
    stripped = text.strip()
    if stripped.startswith("```"):
        stripped = stripped.split("\n", 1)[1].rsplit("```", 1)[0]
    try:
        result = json.loads(stripped)
        if not isinstance(result, dict) or not isinstance(result.get("agentName"), str):
            raise ValueError()
        return result
    except (ValueError, TypeError, IndexError):
        raise ProviderError("InvalidPlannerResponse") from None


class Agents:
    def __init__(self, settings, contract, client, rag, mcp):
        self.settings, self.contract, self.client, self.rag, self.mcp = settings, contract, client, rag, mcp

    async def classify(self, question, previous_summary):
        user = self.contract.text("classifyUser", question=question, previousSummary=previous_summary)
        input_guardrail(user)
        user += self.contract.prompts["enumSuffix"]
        message = await self.client.complete(self.settings.routing_model, [
            {"role": "system", "content": self.contract.prompts["classifySystem"]},
            {"role": "user", "content": user},
        ])
        text = message.get("content")
        if not isinstance(text, str):
            raise ProviderError("InvalidIntent")
        return parse_intent(text)

    async def expert(self, intent, question, previous_summary, remote=False):
        user = self.contract.text("expertUser", question=question, previousSummary=previous_summary)
        contents = await self.rag.retrieve("returns" if intent == "RETURN" else "orders", user)
        augmented = self.contract.text("ragTemplate", userMessage=user, contents="\n\n".join(contents)) if contents else user
        input_guardrail(augmented)
        messages = [
            {"role": "system", "content": self.contract.data["expertPrefixes"][intent] + self.contract.prompts["expertSystem"]},
            {"role": "user", "content": augmented},
        ]
        tools = await self.mcp.tools() if remote else [self.contract.data["tools"]["local"]]
        limit = self.contract.data["limits"]["maxToolLoopIterations"]
        message = await self.client.complete(self.settings.expert_model, messages, tools)
        for _ in range(limit):
            if not message.get("tool_calls"):
                answer_guardrail(message)
                return message["content"]
            messages.append(assistant_message(message))
            # Executa as ferramentas em sequência para preservar a ordem solicitada pelo modelo.
            for call in message["tool_calls"]:
                function = call.get("function") or {}
                if function.get("name") != "findOrder":
                    raise ProviderError("UnknownTool")
                try:
                    arguments = json.loads(function.get("arguments", "{}"))
                    if not isinstance(arguments, dict):
                        raise ValueError()
                except (ValueError, TypeError):
                    raise ProviderError("InvalidToolArguments") from None
                result = await self.mcp.call("findOrder", arguments) if remote else query_order(arguments.get("orderId"))
                messages.append({"role": "tool", "content": result, "tool_call_id": call["id"]})
            message = await self.client.complete(self.settings.expert_model, messages, tools)
        raise ProviderError("ToolLoopLimit")

    async def summarize(self, question, previous_summary, answer):
        message = await self.client.complete(self.settings.routing_model, [
            {"role": "system", "content": self.contract.prompts["summarySystem"]},
            {"role": "user", "content": self.contract.text("summaryUser", previousSummary=previous_summary, question=question, answer=answer)},
        ])
        summary_guardrail(message)
        return message["content"]

    async def supervise(self, question, previous_summary, state=None):
        spec = self.contract.data["supervisor"]
        system = self.contract.text("plannerSystem", agents=spec["agents"], supervisorContext=spec["supervisorContext"])
        original = self.contract.text("supervisorRequest", question=question, previousSummary=previous_summary)
        history = [{"role": "system", "content": system}]
        last_response = ""
        state = state if state is not None else {"question": question, "previousSummary": previous_summary}
        for _ in range(self.contract.data["limits"]["maxSubagentInvocations"]):
            user = self.contract.text("plannerUser", request=original, lastResponse=last_response) + self.contract.prompts["plannerSuffix"]
            history.append({"role": "user", "content": user})
            # Janela por execução; não há memória compartilhada entre requisições.
            if len(history) > self.contract.data["limits"]["chatMemoryMessages"]:
                history = [history[0]] + history[-(self.contract.data["limits"]["chatMemoryMessages"] - 1):]
            message = await self.client.complete(self.settings.expert_model, history)
            history.append(assistant_message(message))
            plan = parse_plan(message.get("content"))
            arguments = plan.get("arguments") or {}
            if not isinstance(arguments, dict):
                raise ProviderError("InvalidPlannerArguments")
            if plan["agentName"].lower() == "done":
                return java_string(arguments["response"]) if arguments.get("response") is not None else last_response
            matched = next((intent for intent, agent_id in spec["agentIds"].items()
                            if plan["agentName"] in (agent_id, "OrderAgent" if intent == "ORDER" else "ReturnAgent")), None)
            if matched is None:
                raise ProviderError("UnknownAgent")
            for key in ("question", "previousSummary"):
                if key in arguments and arguments[key] is None:
                    raise ProviderError("InvalidPlannerArguments")
                if isinstance(arguments.get(key), str):
                    state[key] = arguments[key]
            last_response = await self.expert(matched, state["question"], state["previousSummary"])
        return last_response
