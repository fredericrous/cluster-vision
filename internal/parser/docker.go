package parser

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/fredericrous/cluster-vision/internal/model"
	"gopkg.in/yaml.v3"
)

// dockerComposeFile represents the top-level docker-compose YAML structure.
type dockerComposeFile struct {
	Services map[string]dockerServiceDef `yaml:"services"`
}

type dockerServiceDef struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Hostname      string            `yaml:"hostname"`
	Command       interface{}       `yaml:"command"` // string or []string
	Privileged    bool              `yaml:"privileged"`
	Ports         []string          `yaml:"ports"`
	Volumes       []string          `yaml:"volumes"`
	Networks      map[string]dockerNetworkConfig `yaml:"networks"`
}

type dockerNetworkConfig struct {
	IPv4Address string `yaml:"ipv4_address"`
}

// ParseDockerCompose parses a docker-compose YAML file into a DockerCompose model.
func ParseDockerCompose(data []byte) (*model.DockerCompose, error) {
	var file dockerComposeFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing docker-compose: %w", err)
	}

	if len(file.Services) == 0 {
		slog.Warn("docker-compose file has no services")
		return nil, nil
	}

	// Sort service names for deterministic output
	names := make([]string, 0, len(file.Services))
	for name := range file.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	var services []model.DockerService
	for _, name := range names {
		def := file.Services[name]
		svc := model.DockerService{
			Name:       name,
			Image:      def.Image,
			Hostname:   def.Hostname,
			Ports:      def.Ports,
			Volumes:    def.Volumes,
			Privileged: def.Privileged,
		}

		if svc.Hostname == "" {
			svc.Hostname = def.ContainerName
		}

		// Command — can be string or []string
		switch cmd := def.Command.(type) {
		case string:
			svc.Command = cmd
		case []interface{}:
			parts := make([]string, len(cmd))
			for i, v := range cmd {
				parts[i] = fmt.Sprintf("%v", v)
			}
			svc.Command = fmt.Sprintf("%v", parts)
		}

		// Networks with static IPs
		for netName, netCfg := range def.Networks {
			svc.Networks = append(svc.Networks, netName)
			if netCfg.IPv4Address != "" {
				svc.IP = netCfg.IPv4Address
			}
		}
		sort.Strings(svc.Networks)

		services = append(services, svc)
	}

	return &model.DockerCompose{Services: services}, nil
}
