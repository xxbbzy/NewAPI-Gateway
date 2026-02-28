package service

import "testing"

func TestExtractUsageAndModelFromJSONWithMultipleShapes(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		expectIn    int
		expectOut   int
		expectModel string
		expectSrc   string
		expectParse string
	}{
		{
			name:        "top-level usage map",
			body:        `{"model":"gpt-5.3-codex","usage":{"prompt_tokens":123,"completion_tokens":45}}`,
			expectIn:    123,
			expectOut:   45,
			expectModel: "gpt-5.3-codex",
			expectSrc:   "exact",
			expectParse: "usage-map",
		},
		{
			name:        "message usage map",
			body:        `{"message":{"model":"claude-opus-4-6","usage":{"input_tokens":321,"output_tokens":8}}}`,
			expectIn:    321,
			expectOut:   8,
			expectModel: "claude-opus-4-6",
			expectSrc:   "exact",
			expectParse: "message-usage-map",
		},
		{
			name:        "token_usage map",
			body:        `{"model":"glm-5","token_usage":{"prompt_tokens":9,"completion_tokens":4}}`,
			expectIn:    9,
			expectOut:   4,
			expectModel: "glm-5",
			expectSrc:   "exact",
			expectParse: "token-usage-map",
		},
		{
			name:        "top-level token fields",
			body:        `{"model":"deepseek-chat","input_tokens":5,"output_tokens":69}`,
			expectIn:    5,
			expectOut:   69,
			expectModel: "deepseek-chat",
			expectSrc:   "exact",
			expectParse: "top-level-usage-fields",
		},
		{
			name:        "recursive usage map",
			body:        `{"model":"gpt-5.2","meta":{"stats":{"tokens":{"prompt_tokens":11,"completion_tokens":7}}}}`,
			expectIn:    11,
			expectOut:   7,
			expectModel: "gpt-5.2",
			expectSrc:   "exact",
			expectParse: "recursive-usage-map",
		},
		{
			name:        "missing usage",
			body:        `{"model":"gpt-5.1-codex","choices":[{"message":{"content":"ok"}}]}`,
			expectIn:    0,
			expectOut:   0,
			expectModel: "gpt-5.1-codex",
			expectSrc:   "missing",
			expectParse: "none",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage := extractUsageAndModelFromJSON([]byte(tc.body))
			if usage.PromptTokens != tc.expectIn || usage.CompletionTokens != tc.expectOut {
				t.Fatalf("unexpected tokens: got in=%d out=%d want in=%d out=%d", usage.PromptTokens, usage.CompletionTokens, tc.expectIn, tc.expectOut)
			}
			if usage.ModelName != tc.expectModel {
				t.Fatalf("unexpected model: got=%q want=%q", usage.ModelName, tc.expectModel)
			}
			if usage.UsageSource != tc.expectSrc {
				t.Fatalf("unexpected usage source: got=%q want=%q", usage.UsageSource, tc.expectSrc)
			}
			if usage.UsageParser != tc.expectParse {
				t.Fatalf("unexpected parser: got=%q want=%q", usage.UsageParser, tc.expectParse)
			}
		})
	}
}

func TestExtractUsageAndModelFromSSELineParsesDataLine(t *testing.T) {
	usage, ok := extractUsageAndModelFromSSELine(`data: {"model":"deepseek-chat","usage":{"input_tokens":6,"output_tokens":10}}`)
	if !ok {
		t.Fatalf("expected SSE parser to accept data line")
	}
	if usage.PromptTokens != 6 || usage.CompletionTokens != 10 {
		t.Fatalf("unexpected parsed SSE usage: %+v", usage)
	}
	if usage.UsageSource != "exact" {
		t.Fatalf("expected exact usage source, got %q", usage.UsageSource)
	}
}

func TestMergeUsageMetricsPrefersLargerAndExactValues(t *testing.T) {
	base := usageMetrics{PromptTokens: 5, CompletionTokens: 2, UsageSource: "missing", UsageParser: "none"}
	current := usageMetrics{PromptTokens: 9, CompletionTokens: 1, CacheTokens: 4, UsageSource: "exact", UsageParser: "usage-map"}

	merged := mergeUsageMetrics(base, current)
	if merged.PromptTokens != 9 || merged.CompletionTokens != 2 || merged.CacheTokens != 4 {
		t.Fatalf("unexpected merged metrics: %+v", merged)
	}
	if merged.UsageSource != "exact" || merged.UsageParser != "usage-map" {
		t.Fatalf("unexpected merged source/parser: %+v", merged)
	}
}
