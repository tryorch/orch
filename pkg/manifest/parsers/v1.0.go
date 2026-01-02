package manifestparsers

import (
	"gopkg.in/yaml.v3"
	manifestcore "orch.io/pkg/manifest/core"
)

type V1Manifest struct {
	Version    string                                 `yaml:"version"`
	Inputs     map[string]manifestcore.Input          `yaml:"inputs,omitempty"`
	Metadata   manifestcore.Metadata                  `yaml:"metadata,omitempty"`
	Runners    map[string]manifestcore.RunnerManifest `yaml:"runners"`
	Components map[string]manifestcore.Component      `yaml:"components"`
}

type V1Parser struct{}

func (p *V1Parser) Parse(data []byte) (*manifestcore.Manifest, error) {

	var m V1Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	components := make([]manifestcore.Component, 0, len(m.Components))
	for n, c := range m.Components {
		c.Name = n
		components = append(components, c)
	}

	return &manifestcore.Manifest{
		Version:    m.Version,
		Inputs:     m.Inputs,
		Metadata:   m.Metadata,
		Runners:    m.Runners,
		Components: components,
	}, nil
}
