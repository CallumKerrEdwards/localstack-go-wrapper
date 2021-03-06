package localstack

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/callumkerredwards/localstack-go-wrapper/localstack/services"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

//ImageName provides the docker image name for LocalStack
const ImageName = "docker.io/localstack/localstack"

// Localstack provides methods for starting and stoping a docker container of the latest
// LocalStack image
type Localstack struct {
	ContainerID     string
	dockerClient    *client.Client
	dockerContext   context.Context
	containerConfig *container.Config
	hostConfig      *container.HostConfig
}

// New pulls the latest LocalStack image, and creates a new container. It returns a new
// instance of LocalStack
func New(cfgs ...*services.ServiceConfig) (*Localstack, error) {

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	if err = pullImage(dockerClient, ImageName); err != nil {
		return nil, err
	}

	containerCfg := containerConfig(ImageName, cfgs)
	c, err := json.MarshalIndent(containerCfg, "", "  ")
	if err == nil {
		log.Printf("Container config is %s", string(c))
	}
	hostCfg, err := hostConfig(cfgs)
	if err != nil {
		return nil, err
	}
	h, err := json.MarshalIndent(hostCfg.PortBindings, "", "  ")
	if err == nil {
		log.Printf("Host Config port bindings are %s", string(h))
	}

	ctx := context.Background()
	resp, err := dockerClient.ContainerCreate(ctx, containerCfg, hostCfg, nil, "")
	if err != nil {
		return nil, err
	}

	return &Localstack{resp.ID, dockerClient, ctx, containerCfg, hostCfg}, nil

}

// Start starts the LocalStack container
func (l Localstack) Start() error {
	if err := l.dockerClient.ContainerStart(l.dockerContext, l.ContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	duration := time.Second * 5
	time.Sleep(duration)
	return nil
}

// Stop stops the LocalStack container
func (l Localstack) Stop() error {
	if err := l.dockerClient.ContainerStop(l.dockerContext, l.ContainerID, nil); err != nil {
		return err
	}
	duration := time.Second * 5
	time.Sleep(duration)
	return nil
}

func pullImage(dockerClient *client.Client, img string) error {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	_, err = dockerClient.ImagePull(ctx, img, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	out, err := dockerClient.ImagePull(ctx, img, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	if _, err = io.Copy(os.Stdout, out); err != nil {
		return err
	}
	log.Printf("Successfully pulled image %s", img)
	return nil
}

func containerConfig(img string, serviceConfigs []*services.ServiceConfig) *container.Config {
	conf := &container.Config{
		Image: img,
	}
	sb := strings.Builder{}
	if len(serviceConfigs) > 0 {
		sb.WriteString("SERVICES=")
		names := make([]string, 0, len(serviceConfigs))
		for _, s := range serviceConfigs {
			names = append(names, s.Service.String())
		}
		sb.WriteString(strings.ToLower(strings.Join(names, ",")))
		conf.Env = []string{sb.String()}
	}
	return conf

}

func hostConfig(serviceConfigs []*services.ServiceConfig) (*container.HostConfig, error) {
	m := make(map[nat.Port][]nat.PortBinding)
	if len(serviceConfigs) > 0 {
		for _, s := range serviceConfigs {
			internalPort, binding, err := getMappingForServiceConfig(s)
			if err != nil {
				return nil, err
			}
			m[internalPort] = binding
		}
	} else {
		for _, s := range services.SupportedServices {
			internalPort, binding, err := getMappingForService(s)
			if err != nil {
				return nil, err
			}
			m[internalPort] = binding
		}
	}
	return &container.HostConfig{PortBindings: m}, nil
}

func getMappingForServiceConfig(cfg *services.ServiceConfig) (nat.Port, []nat.PortBinding, error) {
	def, err := services.GetDefaultPort(cfg.Service)
	if err != nil {
		return "nil", nil, err
	}
	port := cfg.Port
	if cfg.Port < 1 {
		port = def
	}
	internalPort, err := nat.NewPort("tcp", strconv.Itoa(def))
	if err != nil {
		return "nil", nil, err
	}
	return internalPort, []nat.PortBinding{
		{
			HostIP:   "0.0.0.0",
			HostPort: strconv.Itoa(port),
		},
	}, nil
}

func getMappingForService(s services.Service) (nat.Port, []nat.PortBinding, error) {
	p, err := services.GetDefaultPort(s)
	if err != nil {
		return "nil", nil, err
	}
	internalPort, err := nat.NewPort("tcp", strconv.Itoa(p))
	if err != nil {
		return "nil", nil, err
	}
	return internalPort, []nat.PortBinding{
		{
			HostIP:   "0.0.0.0",
			HostPort: strconv.Itoa(p),
		},
	}, nil
}
