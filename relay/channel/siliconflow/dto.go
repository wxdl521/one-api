package siliconflow

import "github.com/QuantumNous/the-one/relaykit/dto"

type SFTokens struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type SFMeta struct {
	Tokens SFTokens `json:"tokens"`
}

type SFRerankResponse struct {
	Results []dto.RerankResponseResult `json:"results"`
	Meta    SFMeta                     `json:"meta"`
}

type SFImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	ImageSize      string `json:"image_size,omitempty"`
	// BatchSize/NumInferenceSteps 为计数/步数，0 非有效取值：0 表示未设置，交由上游取默认；
	// BatchSize 另在 adaptor 中以 0 作为回退到 request.N 的哨兵，故保持非指针 omitempty。
	BatchSize uint `json:"batch_size,omitempty"`
	// Seed=0 是有效的确定性种子，必须原样透传，故用指针 + omitempty 保留显式 0。
	Seed              *uint64  `json:"seed,omitempty"`
	NumInferenceSteps uint     `json:"num_inference_steps,omitempty"`
	GuidanceScale     *float64 `json:"guidance_scale,omitempty"`
	Cfg               *float64 `json:"cfg,omitempty"`
	Image             string   `json:"image,omitempty"`
	Image2            string   `json:"image2,omitempty"`
	Image3            string   `json:"image3,omitempty"`
}
