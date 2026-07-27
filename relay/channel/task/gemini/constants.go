package gemini

// TextToVideoModelList is the Veo subset that accepts a plain text prompt.
var TextToVideoModelList = []string{
	"veo-3.0-generate-001",
	"veo-3.0-fast-generate-001",
	"veo-3.1-generate-001",
	"veo-3.1-fast-generate-001",
	"veo-3.1-generate-preview",
	"veo-3.1-fast-generate-preview",
}

// ImageToVideoModelList is the Veo subset that accepts one input reference
// image in addition to the prompt.
var ImageToVideoModelList = TextToVideoModelList
