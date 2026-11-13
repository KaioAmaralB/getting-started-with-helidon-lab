package rag

import (
	"context"
	"encoding/json"
	"fmt"
	ort "github.com/yalue/onnxruntime_go"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

var environmentOnce sync.Once
var environmentError error

type Embedder struct {
	Tokenizer  *Tokenizer
	Session    *ort.DynamicAdvancedSession
	InputNames []string
}

func NewEmbedder(assets, library string) (*Embedder, error) {
	if library == "" {
		name := "libonnxruntime.so"
		if runtime.GOOS == "windows" {
			name = "onnxruntime.dll"
		}
		library = filepath.Join(filepath.Dir(assets), ".tools", "onnxruntime", name)
	}
	environmentOnce.Do(func() { ort.SetSharedLibraryPath(library); environmentError = ort.InitializeEnvironment() })
	if environmentError != nil {
		return nil, fmt.Errorf("inicialização ONNX: %w", environmentError)
	}
	tokenizer, err := NewTokenizer(filepath.Join(assets, "tokenizer.json"))
	if err != nil {
		return nil, err
	}
	model := filepath.Join(assets, "model.onnx")
	inputs, outputs, err := ort.GetInputOutputInfo(model)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(inputs))
	for i, input := range inputs {
		names[i] = input.Name
		if input.Name != "input_ids" && input.Name != "attention_mask" && input.Name != "token_type_ids" {
			return nil, fmt.Errorf("entrada ONNX não suportada")
		}
	}
	if len(outputs) < 1 {
		return nil, fmt.Errorf("saída ONNX ausente")
	}
	session, err := ort.NewDynamicAdvancedSession(model, names, []string{outputs[0].Name}, nil)
	if err != nil {
		return nil, err
	}
	return &Embedder{tokenizer, session, names}, nil
}
func (e *Embedder) Close() error { return e.Session.Destroy() }

func (e *Embedder) Embed(text string) ([]float32, error) {
	parts := partition(e.Tokenizer.Tokens(text))
	if len(parts) == 0 {
		return nil, fmt.Errorf("texto vazio para embedding")
	}
	var aggregate []float32
	total := 0
	for _, part := range parts {
		vector, err := e.infer(e.Tokenizer.EncodePartition(part))
		if err != nil {
			return nil, err
		}
		if len(parts) == 1 {
			aggregate = vector
			total = 1
			break
		}
		if aggregate == nil {
			aggregate = make([]float32, len(vector))
		}
		for j, value := range vector {
			aggregate[j] = float32(aggregate[j] + float32(value*float32(len(part))))
		}
		total += len(part)
	}
	for j := range aggregate {
		aggregate[j] = float32(aggregate[j] / float32(total))
	}
	var square float32
	for _, value := range aggregate {
		square = float32(square + float32(value*value))
	}
	norm := float32(math.Sqrt(float64(square)))
	for j := range aggregate {
		aggregate[j] = float32(aggregate[j] / norm)
	}
	return aggregate, nil
}
func (e *Embedder) infer(ids []int64) ([]float32, error) {
	shape := ort.NewShape(1, int64(len(ids)))
	inputs := make([]ort.Value, len(e.InputNames))
	for i, name := range e.InputNames {
		data := make([]int64, len(ids))
		for j := range data {
			switch name {
			case "input_ids":
				data[j] = ids[j]
			case "attention_mask":
				data[j] = 1
			}
		}
		tensor, err := ort.NewTensor(shape, data)
		if err != nil {
			for _, value := range inputs {
				if value != nil {
					value.Destroy()
				}
			}
			return nil, err
		}
		inputs[i] = tensor
	}
	defer func() {
		for _, value := range inputs {
			value.Destroy()
		}
	}()
	outputs := []ort.Value{nil}
	if err := e.Session.Run(inputs, outputs); err != nil {
		return nil, err
	}
	defer outputs[0].Destroy()
	tensor, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("saída ONNX não é float32")
	}
	dimensions := tensor.GetShape()
	if len(dimensions) != 3 || dimensions[1] != int64(len(ids)) {
		return nil, fmt.Errorf("forma de saída ONNX inesperada")
	}
	width := int(dimensions[2])
	values := tensor.GetData()
	pooled := make([]float32, width)
	for token := range ids {
		for j := 0; j < width; j++ {
			pooled[j] = float32(pooled[j] + values[token*width+j])
		}
	}
	for j := range pooled {
		pooled[j] = float32(pooled[j] / float32(len(ids)))
	}
	return pooled, nil
}

type entry struct {
	ID        string `json:"id"`
	Embedding struct {
		Vector []float32 `json:"vector"`
	} `json:"embedding"`
	Embedded struct {
		Text string `json:"text"`
	} `json:"embedded"`
}
type Retriever struct {
	Embedder *Embedder
	Stores   map[string][]entry
}

func NewRetriever(root, assets, library string) (*Retriever, error) {
	embedder, err := NewEmbedder(assets, library)
	if err != nil {
		return nil, err
	}
	result := &Retriever{embedder, map[string][]entry{}}
	for _, topic := range []string{"orders", "returns"} {
		data, err := os.ReadFile(filepath.Join(root, "knowledge", topic+"-embeddings.json"))
		if err != nil {
			embedder.Close()
			return nil, err
		}
		var store struct {
			Entries []entry `json:"entries"`
		}
		if err = json.Unmarshal(data, &store); err != nil {
			embedder.Close()
			return nil, err
		}
		result.Stores[topic] = store.Entries
	}
	return result, nil
}
func (r *Retriever) Close() error { return r.Embedder.Close() }
func (r *Retriever) Retrieve(ctx context.Context, topic, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vector, err := r.Embedder.Embed(query)
	if err != nil {
		return nil, err
	}
	type match struct {
		text  string
		score float64
	}
	matches := []match{}
	for _, entry := range r.Stores[topic] {
		score := (cosine(vector, entry.Embedding.Vector) + 1) / 2
		if score >= 0 {
			matches = append(matches, match{entry.Embedded.Text, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	result := []string{}
	for i, entry := range matches {
		if i >= 2 {
			break
		}
		result = append(result, entry.text)
	}
	return result, nil
}
func cosine(a, b []float32) float64 {
	var dot, normA, normB float64
	for i, x := range a {
		dot += float64(float32(x * b[i]))
		normA += float64(float32(x * x))
		normB += float64(float32(b[i] * b[i]))
	}
	denominator := math.Sqrt(normA * normB)
	if denominator < 1e-8 {
		denominator = 1e-8
	}
	return dot / denominator
}
