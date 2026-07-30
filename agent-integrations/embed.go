package agentintegrations

import (
	"archive/zip"
	"bytes"
	_ "embed"
)

const MyAgentsSkillVersion = "2026-07-30"

//go:embed myagents/the-one-myagents-pairing/SKILL.md
var MyAgentsPairingSkill string

//go:embed myagents/the-one-gateway/SKILL.md
var MyAgentsGatewaySkill string

//go:embed hermes/the-one-hermes-pairing/SKILL.md
var HermesPairingSkill string

//go:embed hermes/the-one-gateway/SKILL.md
var HermesGatewaySkill string

// MyAgentsGatewaySkillArchive packages the usage Skill in the directory layout
// accepted by MyAgents' built-in Skill installer.
func MyAgentsGatewaySkillArchive() ([]byte, error) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("the-one-gateway/SKILL.md")
	if err != nil {
		return nil, err
	}
	if _, err := file.Write([]byte(MyAgentsGatewaySkill)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}
