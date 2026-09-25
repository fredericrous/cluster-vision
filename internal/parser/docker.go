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
	Image         string         `yaml:"image"`
	ContainerName string         `yaml:"container_name"`
	Hostname      string         `yaml:"hostname"`
	Command       interface{}    `yaml:"command"` // string or []string
	Privileged    bool           `yaml:"privileged"`
	Ports         composePorts   `yaml:"ports"`
	Volumes       composeVolumes `yaml:"volumes"`
	Networks      composeNetwork `yaml:"networks"`
}

type dockerNetworkConfig struct {
	IPv4Address string `yaml:"ipv4_address"`
}

// composeNetwork accepts both forms Compose allows for a service's
// networks: a list of names (`networks: [front]`) or a map of name to
// settings (`networks: {front: {ipv4_address: …}}`).
type composeNetwork map[string]dockerNetworkConfig

func (n *composeNetwork) UnmarshalYAML(node *yaml.Node) error {
	out := composeNetwork{}
	switch node.Kind {
	case yaml.SequenceNode:
		var names []string
		if err := node.Decode(&names); err != nil {
			return err
		}
		for _, name := range names {
			out[name] = dockerNetworkConfig{}
		}
	default:
		var m map[string]*dockerNetworkConfig // a value may be null
		if err := node.Decode(&m); err != nil {
			return err
		}
		for name, cfg := range m {
			if cfg == nil {
				cfg = &dockerNetworkConfig{}
			}
			out[name] = *cfg
		}
	}
	*n = out
	return nil
}

// composePorts accepts the short syntax ("8080:80/udp", or a bare number)
// and the long one ({target, published, protocol, host_ip}), rendering
// both in the short form.
type composePorts []string

func (ps *composePorts) UnmarshalYAML(node *yaml.Node) error {
	var items []yaml.Node
	if err := node.Decode(&items); err != nil {
		return err
	}
	out := make(composePorts, 0, len(items))
	for i := range items {
		item := &items[i]
		if item.Kind == yaml.ScalarNode {
			out = append(out, item.Value)
			continue
		}
		var long struct {
			Target    string `yaml:"target"`
			Published string `yaml:"published"`
			Protocol  string `yaml:"protocol"`
			HostIP    string `yaml:"host_ip"`
		}
		if err := item.Decode(&long); err != nil {
			return err
		}
		port := long.Target
		if long.Published != "" {
			port = long.Published + ":" + port
			if long.HostIP != "" {
				port = long.HostIP + ":" + port
			}
		}
		if long.Protocol != "" && long.Protocol != "tcp" {
			port += "/" + long.Protocol
		}
		out = append(out, port)
	}
	*ps = out
	return nil
}

// composeVolumes accepts the short syntax ("./data:/data:ro") and the long
// one ({type, source, target, read_only}), rendering both in the short form.
type composeVolumes []string

func (vs *composeVolumes) UnmarshalYAML(node *yaml.Node) error {
	var items []yaml.Node
	if err := node.Decode(&items); err != nil {
		return err
	}
	out := make(composeVolumes, 0, len(items))
	for i := range items {
		item := &items[i]
		if item.Kind == yaml.ScalarNode {
			out = append(out, item.Value)
			continue
		}
		var long struct {
			Source   string `yaml:"source"`
			Target   string `yaml:"target"`
			ReadOnly bool   `yaml:"read_only"`
		}
		if err := item.Decode(&long); err != nil {
			return err
		}
		vol := long.Target
		if long.Source != "" {
			vol = long.Source + ":" + vol
		}
		if long.ReadOnly {
			vol += ":ro"
		}
		out = append(out, vol)
	}
	*vs = out
	return nil
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
			Ports:      []string(def.Ports),
			Volumes:    []string(def.Volumes),
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

		// Networks with static IPs. Walk them in name order: with several
		// static addresses the first network's wins, not a random one's.
		for netName := range def.Networks {
			svc.Networks = append(svc.Networks, netName)
		}
		sort.Strings(svc.Networks)
		for _, netName := range svc.Networks {
			if ip := def.Networks[netName].IPv4Address; ip != "" {
				svc.IP = ip
				break
			}
		}

		services = append(services, svc)
	}

	return &model.DockerCompose{Services: services}, nil
}
