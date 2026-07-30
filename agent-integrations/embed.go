package agentintegrations

import _ "embed"

const MyAgentsSkillVersion = "2026-07-30"

//go:embed myagents/the-one-myagents-pairing/SKILL.md
var MyAgentsPairingSkill string

//go:embed myagents/the-one-gateway/SKILL.md
var MyAgentsGatewaySkill string
