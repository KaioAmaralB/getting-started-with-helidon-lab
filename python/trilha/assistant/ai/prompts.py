"""Prompts compartilhados e renderização literal das mensagens."""
import json
import re


class PromptContract:
    def __init__(self, lab_root):
        self.data = json.loads((lab_root / "comparison" / "contract.json").read_text(encoding="utf-8-sig"))
        self.prompts = self.data["prompts"]

    @staticmethod
    def render(template, **values):
        def replacement(match):
            key = match.group(1).strip("".join(map(chr, range(33))))
            if key not in values or values[key] is None:
                raise ValueError("MissingPromptVariable")
            return values[key]
        return re.sub(r"{{\s*(.+?)\s*}}", replacement, template)

    def text(self, name, **values):
        return self.render(self.prompts[name], **values)
