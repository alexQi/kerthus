package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeAgentProvidersPreservesMaskedCredentialsByCode(t *testing.T) {
	old := `[{"code":"openai","provider":"openai","api_key":"secret-a","model":"old"},{"code":"deepseek","api_key":"secret-b"}]`
	incoming := `[{"code":"deepseek","api_key":"********","model":"new"},{"code":"openai","api_key":"","model":"new"}]`
	got, err := mergeAgentProviders(old, incoming)
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(got), &list); err != nil {
		t.Fatal(err)
	}
	if list[0]["api_key"] != "secret-b" || list[1]["api_key"] != "secret-a" {
		t.Fatalf("provider credentials were not matched by code: %#v", list)
	}
}

func TestMergeAgentProvidersPreservesMaskedCredentialsByIndex(t *testing.T) {
	old := `[{"provider":"one","api_key":"secret-a"},{"provider":"two","api_key":"secret-b"}]`
	incoming := `[{"provider":"renamed-one","api_key":"********"},{"provider":"renamed-two","api_key":""}]`
	got, err := mergeAgentProviders(old, incoming)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"api_key":"secret-a"`) || !strings.Contains(got, `"api_key":"secret-b"`) {
		t.Fatalf("provider credentials were not preserved by index: %s", got)
	}
}

func TestMergeAgentProvidersRejectsNonArray(t *testing.T) {
	if _, err := mergeAgentProviders("[]", `{"provider":"openai"}`); err == nil {
		t.Fatal("accepted provider object where array was required")
	}
}

func TestMaskAgentProviders(t *testing.T) {
	raw := `[{"code":"main","api_key":"secret-a","apiKey":"secret-b","model":"gpt"}]`
	got := maskAgentProviders(raw)
	if strings.Contains(got, "secret-a") || strings.Contains(got, "secret-b") {
		t.Fatalf("provider credentials leaked: %s", got)
	}
	if !strings.Contains(got, `"api_key":"********"`) || !strings.Contains(got, `"apiKey":"********"`) {
		t.Fatalf("provider credentials were not masked: %s", got)
	}
}
