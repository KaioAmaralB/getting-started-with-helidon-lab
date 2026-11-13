package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"lab.trilha/go/internal/assistant/config"
	"lab.trilha/go/internal/assistant/guardrail"
	"lab.trilha/go/internal/domain"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Retrieval interface {
	Retrieve(context.Context, string, string) ([]string, error)
}
type RemoteTool interface {
	CallTool(context.Context, string, json.RawMessage) (string, error)
}

type Agents struct {
	Contract     config.Contract
	Model        Completion
	Retrieval    Retrieval
	Remote       RemoteTool
	RoutingModel string
	ExpertModel  string
}
type Request struct {
	Message string `json:"message"`
	Summary string `json:"summary"`
}
type Response struct {
	Answer  string `json:"answer"`
	Summary string `json:"summary"`
}

var placeholders = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

func Render(template string, values map[string]string) string {
	return placeholders.ReplaceAllStringFunc(template, func(match string) string {
		key := strings.TrimSpace(placeholders.FindStringSubmatch(match)[1])
		if value, ok := values[key]; ok {
			return value
		}
		return match
	})
}
func values(question, summary, answer string) map[string]string {
	return map[string]string{"question": question, "previousSummary": summary, "answer": answer}
}
func textResult(message Message) (string, error) {
	text := message.Text()
	if text == nil {
		return "", fmt.Errorf("modelo não retornou texto")
	}
	return *text, nil
}

func (a *Agents) Classify(ctx context.Context, question, summary string) (string, error) {
	user := Render(a.Contract.Prompts["classifyUser"], values(question, summary, ""))
	if err := guardrail.Input(user); err != nil {
		return "", err
	}
	user += a.Contract.Prompts["enumSuffix"]
	result, err := a.Model.Chat(ctx, a.RoutingModel, []Message{{Role: "system", Content: a.Contract.Prompts["classifySystem"]}, {Role: "user", Content: user}}, nil)
	if err != nil {
		return "", err
	}
	text, err := textResult(result)
	if err != nil {
		return "", err
	}
	text = strings.Trim(strings.TrimSpace(text), "\"")
	if text != "ORDER" && text != "RETURN" {
		return "", fmt.Errorf("enum de classificação inválido")
	}
	return text, nil
}

func (a *Agents) Expert(ctx context.Context, intent, question, summary string, remote bool) (string, error) {
	topic := "orders"
	if intent == "RETURN" {
		topic = "returns"
	}
	user := Render(a.Contract.Prompts["expertUser"], values(question, summary, ""))
	contents, err := a.Retrieval.Retrieve(ctx, topic, user)
	if err != nil {
		return "", err
	}
	if len(contents) > 0 {
		user = Render(a.Contract.Prompts["ragTemplate"], map[string]string{"userMessage": user, "contents": strings.Join(contents, "\n\n")})
	}
	if err = guardrail.Input(user); err != nil {
		return "", err
	}
	system := a.Contract.ExpertPrefixes[intent] + a.Contract.Prompts["expertSystem"]
	kind := "local"
	if remote {
		kind = "mcp"
	}
	var tool any
	if err = json.Unmarshal(a.Contract.Tools[kind], &tool); err != nil {
		return "", err
	}
	tools := []any{tool}
	messages := []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	response, err := a.Model.Chat(ctx, a.ExpertModel, messages, tools)
	if err != nil {
		return "", err
	}
	maximum := a.Contract.Limits["maxToolLoopIterations"]
	if maximum == 0 {
		maximum = 100
	}
	for round := 0; round < maximum; round++ {
		if len(response.ToolCalls) == 0 {
			if err = guardrail.Output(response.Text(), false, false); err != nil {
				return "", err
			}
			return textResult(response)
		}
		// Retain the provider's argument string byte-for-byte in the assistant message.
		messages = append(messages, response)
		for _, call := range response.ToolCalls {
			if call.Function.Name != "findOrder" {
				return "", fmt.Errorf("tool desconhecida")
			}
			var observation string
			if remote {
				observation, err = a.Remote.CallTool(ctx, call.Function.Name, json.RawMessage(call.Function.Arguments))
			} else {
				var args struct {
					OrderID string `json:"orderId"`
				}
				if err = json.Unmarshal([]byte(call.Function.Arguments), &args); err == nil {
					observation = domain.QueryOrder(args.OrderID)
				}
			}
			if err != nil {
				return "", err
			}
			messages = append(messages, Message{Role: "tool", Content: observation, ToolCallID: call.ID})
		}
		response, err = a.Model.Chat(ctx, a.ExpertModel, messages, tools)
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("limite de rodadas de tools excedido")
}

func (a *Agents) Summarize(ctx context.Context, question, summary, answer string) (string, error) {
	user := Render(a.Contract.Prompts["summaryUser"], values(question, summary, answer))
	result, err := a.Model.Chat(ctx, a.RoutingModel, []Message{{Role: "system", Content: a.Contract.Prompts["summarySystem"]}, {Role: "user", Content: user}}, nil)
	if err != nil {
		return "", err
	}
	if err = guardrail.Output(result.Text(), false, true); err != nil {
		return "", err
	}
	return textResult(result)
}

func (a *Agents) Workflow(ctx context.Context, request Request) (Response, error) {
	intent, err := a.Classify(ctx, request.Message, request.Summary)
	if err != nil {
		return Response{}, err
	}
	answer, err := a.Expert(ctx, intent, request.Message, request.Summary, false)
	if err != nil {
		return Response{}, err
	}
	summary, err := a.Summarize(ctx, request.Message, request.Summary, answer)
	return Response{answer, summary}, err
}
func (a *Agents) Direct(ctx context.Context, request Request) (Response, error) {
	answer, err := a.Expert(ctx, "RETURN", request.Message, request.Summary, false)
	return Response{Answer: answer, Summary: ""}, err
}
func (a *Agents) MCPWorkflow(ctx context.Context, request Request) (Response, error) {
	answer, err := a.Expert(ctx, "ORDER", request.Message, request.Summary, true)
	if err != nil {
		return Response{}, err
	}
	summary, err := a.Summarize(ctx, request.Message, request.Summary, answer)
	return Response{answer, summary}, err
}

type plan struct {
	AgentName   string          `json:"agentName"`
	Arguments   map[string]any  `json:"arguments"`
	RawResponse json.RawMessage `json:"-"`
}

func parsePlan(text string) (plan, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if newline := strings.IndexByte(text, '\n'); newline >= 0 {
			text = text[newline+1:]
		}
		text = strings.TrimSpace(strings.TrimSuffix(text, "```"))
	}
	var result plan
	err := json.Unmarshal([]byte(text), &result)
	var raw struct {
		Arguments map[string]json.RawMessage `json:"arguments"`
	}
	if err == nil {
		if rawErr := json.Unmarshal([]byte(text), &raw); rawErr == nil {
			result.RawResponse = raw.Arguments["response"]
		}
	}
	if err == nil && result.AgentName == "" {
		err = fmt.Errorf("planner sem agentName")
	}
	return result, err
}

func (a *Agents) SupervisorWorkflow(ctx context.Context, request Request) (Response, error) {
	question, summary := request.Message, request.Summary
	original := Render(a.Contract.Prompts["supervisorRequest"], values(question, summary, ""))
	system := Render(a.Contract.Prompts["plannerSystem"], map[string]string{"agents": a.Contract.Supervisor.Agents, "supervisorContext": a.Contract.Supervisor.Context})
	messages := []Message{{Role: "system", Content: system}}
	last := ""
	answer := ""
	maximum := a.Contract.Limits["maxSubagentInvocations"]
	if maximum == 0 {
		maximum = 3
	}
	for invocation := 0; invocation < maximum; invocation++ {
		user := Render(a.Contract.Prompts["plannerUser"], map[string]string{"request": original, "lastResponse": last}) + a.Contract.Prompts["plannerSuffix"]
		messages = append(messages, Message{Role: "user", Content: user})
		// Retain 20 recent messages plus the system message, as in the Java planner.
		if len(messages) > 21 {
			messages = append([]Message{messages[0]}, messages[len(messages)-20:]...)
		}
		result, err := a.Model.Chat(ctx, a.ExpertModel, messages, nil)
		if err != nil {
			return Response{}, err
		}
		text, err := textResult(result)
		if err != nil {
			return Response{}, err
		}
		next, err := parsePlan(text)
		if err != nil {
			return Response{}, err
		}
		messages = append(messages, result)
		if strings.EqualFold(next.AgentName, "done") {
			answer = last
			if value, ok := next.Arguments["response"]; ok && value != nil {
				if len(next.RawResponse) > 0 {
					answer = javaJSONText(next.RawResponse)
				} else {
					answer = fmt.Sprint(value)
				}
			}
			break
		}
		intent := ""
		for candidate, id := range a.Contract.Supervisor.AgentIDs {
			if next.AgentName == id || next.AgentName == strings.Split(id, "$")[0] {
				intent = candidate
				break
			}
		}
		if intent == "" {
			return Response{}, fmt.Errorf("especialista desconhecido no plano")
		}
		if value, ok := next.Arguments["question"].(string); ok {
			question = value
		}
		if value, ok := next.Arguments["previousSummary"].(string); ok {
			summary = value
		}
		last, err = a.Expert(ctx, intent, question, summary, false)
		if err != nil {
			return Response{}, err
		}
		answer = last
	}
	nextSummary, err := a.Summarize(ctx, question, summary, answer)
	if err != nil {
		return Response{}, err
	}
	if err = guardrail.Output(&answer, false, false); err != nil {
		return Response{}, err
	}
	return Response{answer, nextSummary}, nil
}

// Java's Map<String,Object>.toString is observable when a planner returns a non-string response.
// Preserve object field order and Java's list/map rendering instead of fmt.Sprint(map).
func javaJSONText(raw json.RawMessage) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var read func() string
	read = func() string {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case nil:
			return "null"
		case string:
			return value
		case bool:
			return strconv.FormatBool(value)
		case json.Number:
			text := value.String()
			if !strings.ContainsAny(text, ".eE") {
				return text
			}
			number, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return text
			}
			absolute := math.Abs(number)
			if absolute == 0 || (absolute >= 0.001 && absolute < 1e7) {
				text = strconv.FormatFloat(number, 'f', -1, 64)
				if !strings.Contains(text, ".") {
					text += ".0"
				}
				return text
			}
			parts := strings.Split(strconv.FormatFloat(number, 'e', -1, 64), "e")
			if !strings.Contains(parts[0], ".") {
				parts[0] += ".0"
			}
			exponent, _ := strconv.Atoi(parts[1])
			return parts[0] + "E" + strconv.Itoa(exponent)
		case json.Delim:
			parts := []string{}
			if value == '[' {
				for decoder.More() {
					parts = append(parts, read())
				}
				decoder.Token()
				return "[" + strings.Join(parts, ", ") + "]"
			}
			if value == '{' {
				for decoder.More() {
					key, _ := decoder.Token()
					parts = append(parts, fmt.Sprint(key)+"="+read())
				}
				decoder.Token()
				return "{" + strings.Join(parts, ", ") + "}"
			}
		}
		return ""
	}
	return read()
}
