package ai

import "strings"

type Provider string

const (
	ProviderOpenAI      Provider = "openai"
	ProviderCompatible  Provider = "openai_compatible"
	ProviderOllama      Provider = "ollama"
	ProviderZhipu       Provider = "zhipu"
	ProviderGroq        Provider = "groq"
	ProviderCerebras    Provider = "cerebras"
	ProviderSiliconFlow Provider = "siliconflow"
	ProviderModelScope  Provider = "modelscope"
)

type ProviderPreset struct {
	ID             Provider `json:"id"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	DefaultModel   string   `json:"default_model"`
	APIKeyEnv      string   `json:"api_key_env,omitempty"`
	Website        string   `json:"website"`
	Free           bool     `json:"free"`
	Local          bool     `json:"local"`
	RequiresAPIKey bool     `json:"requires_api_key"`
}

func (p ProviderPreset) ChatCompletionsURL() string {
	return strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
}

var providerPresets = []ProviderPreset{
	{
		ID:             ProviderOpenAI,
		Name:           "OpenAI",
		BaseURL:        "https://api.openai.com/v1",
		DefaultModel:   "gpt-4o-mini",
		APIKeyEnv:      "OPENAI_API_KEY",
		Website:        "https://platform.openai.com/api-keys",
		RequiresAPIKey: true,
	},
	{
		ID:             ProviderCompatible,
		Name:           "OpenAI Compatible",
		BaseURL:        "",
		DefaultModel:   "qwen2.5-7b-instruct",
		Website:        "https://platform.openai.com/docs/api-reference/chat",
		RequiresAPIKey: true,
	},
	{
		ID:           ProviderOllama,
		Name:         "Ollama",
		BaseURL:      "http://localhost:11434/v1",
		DefaultModel: "qwen2.5:7b",
		Website:      "https://ollama.com/download",
		Free:         true,
		Local:        true,
	},
	{
		ID:             ProviderZhipu,
		Name:           "智谱AI",
		BaseURL:        "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel:   "glm-4.7-flash",
		APIKeyEnv:      "ZHIPU_API_KEY",
		Website:        "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
		Free:           true,
		RequiresAPIKey: true,
	},
	{
		ID:             ProviderGroq,
		Name:           "Groq",
		BaseURL:        "https://api.groq.com/openai/v1",
		DefaultModel:   "llama-3.3-70b-versatile",
		APIKeyEnv:      "GROQ_API_KEY",
		Website:        "https://console.groq.com/keys",
		Free:           true,
		RequiresAPIKey: true,
	},
	{
		ID:             ProviderCerebras,
		Name:           "Cerebras Systems",
		BaseURL:        "https://api.cerebras.ai/v1",
		DefaultModel:   "llama3.1-8b",
		APIKeyEnv:      "CEREBRAS_API_KEY",
		Website:        "https://cloud.cerebras.ai/platform",
		Free:           true,
		RequiresAPIKey: true,
	},
	{
		ID:             ProviderSiliconFlow,
		Name:           "硅基流动 SiliconFlow",
		BaseURL:        "https://api.siliconflow.cn/v1",
		DefaultModel:   "Qwen/Qwen2.5-7B-Instruct",
		APIKeyEnv:      "SILICONFLOW_API_KEY",
		Website:        "https://cloud.siliconflow.cn/account/ak",
		Free:           true,
		RequiresAPIKey: true,
	},
	{
		ID:             ProviderModelScope,
		Name:           "魔塔社区 ModelScope",
		BaseURL:        "https://api.modelscope.cn/v1",
		DefaultModel:   "qwen2-7b-instruct",
		APIKeyEnv:      "MODELSCOPE_API_KEY",
		Website:        "https://modelscope.cn/my/myaccesstoken",
		Free:           true,
		RequiresAPIKey: true,
	},
}

func ProviderPresets() []ProviderPreset {
	out := make([]ProviderPreset, len(providerPresets))
	copy(out, providerPresets)
	return out
}

func LookupProvider(id Provider) (ProviderPreset, bool) {
	for _, preset := range providerPresets {
		if preset.ID == id {
			return preset, true
		}
	}
	return ProviderPreset{}, false
}

func DefaultFallbackProviders() []Provider {
	return []Provider{ProviderGroq, ProviderSiliconFlow, ProviderModelScope, ProviderOllama, ProviderOpenAI}
}

func NormalizeProvider(id string) Provider {
	switch Provider(strings.TrimSpace(id)) {
	case ProviderOpenAI, ProviderOllama, ProviderZhipu, ProviderGroq, ProviderCerebras, ProviderSiliconFlow, ProviderModelScope:
		return Provider(strings.TrimSpace(id))
	default:
		return ProviderCompatible
	}
}
