"""Recuperação com MiniLM ONNX e cálculo float32 compatível com LangChain4j."""
import asyncio
from concurrent.futures import ThreadPoolExecutor
import json
import math
from pathlib import Path

import numpy as np
import onnxruntime as ort
from tokenizers import Tokenizer


class PolicyRetriever:
    def __init__(self, settings):
        self.executor = ThreadPoolExecutor(max_workers=settings.rag_workers, thread_name_prefix="trilha-rag")
        assets = settings.lab_root / "comparison" / "assets"
        self.tokenizer = Tokenizer.from_file(str(assets / "tokenizer.json"))
        # Mantém a truncagem de 128 tokens definida em tokenizer.json.
        self.tokenizer.no_padding()
        self.session = ort.InferenceSession(str(assets / "model.onnx"), providers=["CPUExecutionProvider"])
        self.inputs = {item.name for item in self.session.get_inputs()}
        # Os snapshots de políticas são compartilhados a partir de java/knowledge.
        knowledge_root = settings.lab_root / "java" / "knowledge"
        if not knowledge_root.is_dir():
            knowledge_root = settings.lab_root / "knowledge"
        self.stores = {}
        for topic in ("orders", "returns"):
            store = json.loads((knowledge_root / (topic + "-embeddings.json")).read_text(encoding="utf-8-sig"))
            self.stores[topic] = store["entries"]

    def close(self):
        self.executor.shutdown(wait=True)

    def token_ids(self, text):
        return self.tokenizer.encode(text).ids

    @staticmethod
    def _partitions(tokens):
        parts, start = [], 1
        while start < len(tokens) - 1:
            end = start + 510
            if end >= len(tokens) - 1:
                end = len(tokens) - 1
            else:
                while end > start and tokens[end].startswith("##"):
                    end -= 1
                if end == start:
                    end = start + 510
            parts.append(tokens[start:end])
            start = end
        return parts

    def _text_from_tokens(self, tokens):
        # Reproduz buildSentence do DJL 0.36.0, sem o cleanup do decoder Hugging Face.
        text = " ".join(tokens).replace(" ##", "").strip("".join(map(chr, range(33))))
        recovered = self.tokenizer.encode(text).tokens[1:-1]
        return text if recovered == tokens else "".join(tokens)

    def embed(self, text):
        tokens = self.tokenizer.encode(text).tokens
        partitions = self._partitions(tokens)
        if not partitions:
            raise ValueError("Cannot embed empty or whitespace-only text")
        pooled = []
        for partition in partitions:
            encoding = self.tokenizer.encode(self._text_from_tokens(partition))
            arrays = {
                "input_ids": np.asarray([encoding.ids], dtype=np.int64),
                "attention_mask": np.asarray([encoding.attention_mask], dtype=np.int64),
                "token_type_ids": np.asarray([encoding.type_ids], dtype=np.int64),
            }
            vectors = self.session.run(None, {key: value for key, value in arrays.items() if key in self.inputs})[0][0]
            mean = np.zeros(vectors.shape[1], dtype=np.float32)
            for vector in vectors:
                mean += vector
            mean /= np.float32(len(vectors))
            pooled.append(mean)
        if len(pooled) == 1:
            result = pooled[0]
        else:
            result = np.zeros_like(pooled[0])
            for vector, partition in zip(pooled, partitions):
                result += vector * np.float32(len(partition))
            result /= np.float32(sum(map(len, partitions)))
        square_sum = np.float32(0)
        for value in result:
            square_sum = np.float32(square_sum + np.float32(value * value))
        norm = np.float32(math.sqrt(float(square_sum)))
        return result / norm

    def retrieve_sync(self, topic, query):
        query_vector = self.embed(query)
        ranked = []
        for index, entry in enumerate(self.stores[topic]):
            vector = np.asarray(entry["embedding"]["vector"], dtype=np.float32)
            # Converte a similaridade de cosseno para o intervalo [0, 1] usado nos snapshots.
            dot = sum(float(np.float32(a * b)) for a, b in zip(query_vector, vector))
            norm_left = math.sqrt(sum(float(np.float32(a * a)) for a in query_vector))
            norm_right = math.sqrt(sum(float(np.float32(b * b)) for b in vector))
            cosine = dot / max(norm_left * norm_right, float(np.float32(1e-8)))
            score = (cosine + 1.0) / 2.0
            if score >= 0.0:
                ranked.append((score, index, entry["embedded"]["text"]))
        ranked.sort(key=lambda item: (-item[0], item[1]))
        return [entry[2] for entry in ranked[:2]]

    async def retrieve(self, topic, query):
        return await asyncio.get_running_loop().run_in_executor(self.executor, self.retrieve_sync, topic, query)
