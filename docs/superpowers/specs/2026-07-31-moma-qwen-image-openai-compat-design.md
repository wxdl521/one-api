# MoMA Qwen Image 2 OpenAI Images Compatibility

## Goal

Let an advanced custom channel expose MoMA's `qwen/qwen-image-2.0-pro` through the standard OpenAI Images API. Clients will call `POST /v1/images/generations` and receive an OpenAI-compatible image response instead of constructing MoMA's native request payload.

## Scope

- Add one advanced-custom converter dedicated to `qwen/qwen-image-2.0-pro`.
- Convert OpenAI Images generation requests to MoMA's `POST /v1/aigc/multimodal-generation/generation` schema.
- Convert MoMA's successful image result to the existing OpenAI image response DTO.
- Make the converter selectable in the advanced-custom route editor.
- Preserve standard OpenAI image request validation and existing image-count billing bounds.

## Non-goals

- No generic MoMA converter or new channel type.
- No support for MoMA editing, video, multimodal chat, or image models other than `qwen/qwen-image-2.0-pro`.
- No change to native MoMA passthrough routes; they remain supported for callers that already use MoMA's API.

## Request flow

1. A client calls the gateway's `/v1/images/generations` with `model: "qwen/qwen-image-2.0-pro"` and an OpenAI image request body.
2. The advanced-custom route matches both the incoming path and the exact model name.
3. The new converter rejects every other model with a clear invalid-request error.
4. It produces the MoMA request:
   - `model` remains `qwen/qwen-image-2.0-pro`.
   - `prompt` becomes one user text item under `input.messages`.
   - `n`, `size`, and supported OpenAI image options become MoMA `parameters`.
   - Unsupported OpenAI options are rejected rather than silently discarded.
5. The normal advanced-custom authentication mechanism sends `Authorization: Bearer {api_key}` to the MoMA upstream path.
6. The MoMA result is converted into the existing OpenAI Images response shape, including one image data item per generated image URL or base64 payload provided by MoMA.

## Configuration and UI

The route editor will expose a converter named for MoMA Qwen Image 2. Its recommended route template will use:

```json
{
  "incoming_path": "/v1/images/generations",
  "upstream_path": "/v1/aigc/multimodal-generation/generation",
  "converter": "openai_image_to_moma_qwen_image",
  "models": ["qwen/qwen-image-2.0-pro"],
  "auth": {
    "type": "header",
    "name": "Authorization",
    "value": "Bearer {api_key}"
  }
}
```

The existing channel test dialog's image-generation endpoint will then test this model through `/v1/images/generations`, like other OpenAI-compatible image models.

## Error handling

- A route configured with this converter and any model other than the exact supported MoMA model returns a client-visible invalid-request error.
- Unsupported image request fields return a 400 with the field name and supported alternative where applicable.
- Malformed or unsuccessful MoMA responses retain the upstream failure semantics; they must not be reported as successful OpenAI image results.
- The existing request validation limits for `n` remain authoritative before conversion.

## Tests

Backend regression tests will cover:

1. A valid OpenAI image generation request converts to the expected MoMA request body and target path.
2. The converter rejects a non-MoMA model.
3. Unsupported OpenAI image options are rejected.
4. A representative MoMA response becomes a valid OpenAI image response.
5. An advanced-custom route with this converter accepts `/v1/images/generations` and rejects an unrelated incoming path.

Frontend verification will confirm the converter appears in the route editor and serializes the documented converter identifier.
