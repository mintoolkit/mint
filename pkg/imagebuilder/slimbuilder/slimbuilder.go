package slimbuilder

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/consts"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/docker/dockerfile"
	"github.com/mintoolkit/mint/pkg/util/fsutil"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidContextDir = errors.New("invalid context directory")
)

// SlimImageBuilder creates new optimized container images
type SlimImageBuilder struct {
	ShowBuildLogs bool
	BuildOptions  docker.BuildImageOptions
	APIClient     *docker.Client
	BuildLog      bytes.Buffer

	RepoName       string
	AdditionalTags []string
	ID             string
	Entrypoint     []string
	Cmd            []string
	WorkingDir     string
	Env            []string
	Labels         map[string]string
	ExposedPorts   map[docker.Port]struct{}
	Volumes        map[string]struct{}
	OnBuild        []string
	User           string
	HasData        bool
	TarData        bool
}

const (
	dsCmdPortInfo = "65501/tcp"
	dsEvtPortInfo = "65502/tcp"
)

// NewSlimImageBuilder creates a new SlimImageBuilder instances
func NewSlimImageBuilder(
	client *docker.Client,
	imageRepoNameTag string,
	additionalTags []string,
	imageInfo *crt.ImageInfo,
	artifactLocation string,
	showBuildLogs bool,
	overrideSelectors map[string]bool,
	overrides *config.ContainerOverrides,
	instructions *config.ImageNewInstructions,
	sourceImage string) (*SlimImageBuilder, error) {

	labels := map[string]string{}
	if imageInfo.Config.Labels != nil {
		//cleanup non-standard labels from buildpacks
		for k, v := range imageInfo.Config.Labels {
			lineLen := len(k) + len(v) + 7
			if lineLen > 65535 {
				//TODO: improve JSON data splitting
				valueLen := len(v)
				parts := valueLen / 50000
				parts++
				offset := 0
				for i := 0; i < parts && offset < valueLen; i++ {
					chunkSize := 50000
					if (offset + chunkSize) > valueLen {
						chunkSize = valueLen - offset
					}
					value := v[offset:(offset + chunkSize)]
					offset += chunkSize
					key := fmt.Sprintf("%s.%d", k, i)
					labels[key] = value
				}
			} else {
				labels[k] = v
			}
		}
	}

	var platform string
	if imageInfo != nil && imageInfo.OS != "" && imageInfo.Architecture != "" {
		platform = fmt.Sprintf("%s/%s", imageInfo.OS, imageInfo.Architecture)
	}
	// omitempty will remove platform if empty on marshalled request to engine

	builder := &SlimImageBuilder{
		ShowBuildLogs: showBuildLogs,
		APIClient:     client,
		// extract platform - from image inspector
		BuildOptions: docker.BuildImageOptions{
			Name:           imageRepoNameTag,
			RmTmpContainer: true,
			ContextDir:     artifactLocation,
			Dockerfile:     "Dockerfile",
			Platform:       platform,
			//SuppressOutput: true,
		},
		RepoName:       imageRepoNameTag,
		AdditionalTags: additionalTags,
		ID:             imageInfo.ID,
		Entrypoint:     imageInfo.Config.Entrypoint,
		Cmd:            imageInfo.Config.Cmd,
		WorkingDir:     imageInfo.Config.WorkingDir,
		Env:            imageInfo.Config.Env,
		Labels:         labels,
		Volumes:        imageInfo.Config.Volumes,
		OnBuild:        imageInfo.Config.OnBuild,
		User:           imageInfo.Config.User,
	}

	if imageInfo.Config.ExposedPorts != nil {
		builder.ExposedPorts = map[docker.Port]struct{}{}
		for k, v := range imageInfo.Config.ExposedPorts {
			builder.ExposedPorts[docker.Port(k)] = v
		}
	}

	if builder.ExposedPorts == nil {
		builder.ExposedPorts = map[docker.Port]struct{}{}
	}

	if builder.Volumes == nil {
		builder.Volumes = map[string]struct{}{}
	}

	if builder.Labels == nil {
		builder.Labels = map[string]string{}
	}

	if overrides != nil && len(overrideSelectors) > 0 {
		log.Debugf("NewSlimImageBuilder: Using container runtime overrides => %+v", overrideSelectors)
		for k := range overrideSelectors {
			switch k {
			case "entrypoint":
				if len(overrides.Entrypoint) > 0 {
					builder.Entrypoint = overrides.Entrypoint
				}
			case "cmd":
				if len(overrides.Cmd) > 0 {
					builder.Cmd = overrides.Cmd
				}
			case "workdir":
				if overrides.Workdir != "" {
					builder.WorkingDir = overrides.Workdir
				}
			case "env":
				if len(overrides.Env) > 0 {
					builder.Env = append(builder.Env, overrides.Env...)
				}
			case "label":
				for k, v := range overrides.Labels {
					builder.Labels[k] = v
				}
			case "volume":
				for k, v := range overrides.Volumes {
					builder.Volumes[k] = v
				}
			case "expose":
				dsCmdPort := docker.Port(dsCmdPortInfo)
				dsEvtPort := docker.Port(dsEvtPortInfo)

				for k, v := range overrides.ExposedPorts {
					if k == dsCmdPort || k == dsEvtPort {
						continue
					}
					builder.ExposedPorts[k] = v
				}
			}
		}
	}

	//instructions have higher value precedence over the runtime overrides
	if instructions != nil {
		log.Debugf("NewSlimImageBuilder: Using new image instructions => %+v", instructions)

		if instructions.Workdir != "" {
			builder.WorkingDir = instructions.Workdir
		}

		if len(instructions.Env) > 0 {
			builder.Env = append(builder.Env, instructions.Env...)
		}

		for k, v := range instructions.ExposedPorts {
			builder.ExposedPorts[k] = v
		}

		for k, v := range instructions.Volumes {
			builder.Volumes[k] = v
		}

		for k, v := range instructions.Labels {
			builder.Labels[k] = v
		}

		if len(instructions.Entrypoint) > 0 {
			builder.Entrypoint = instructions.Entrypoint
		}

		if len(instructions.Cmd) > 0 {
			builder.Cmd = instructions.Cmd
		}

		if len(builder.ExposedPorts) > 0 &&
			len(instructions.RemoveExposedPorts) > 0 {
			for k := range instructions.RemoveExposedPorts {
				if _, ok := builder.ExposedPorts[k]; ok {
					delete(builder.ExposedPorts, k)
				}
			}
		}

		if len(builder.Volumes) > 0 &&
			len(instructions.RemoveVolumes) > 0 {
			for k := range instructions.RemoveVolumes {
				if _, ok := builder.Volumes[k]; ok {
					delete(builder.Volumes, k)
				}
			}
		}

		if len(builder.Labels) > 0 &&
			len(instructions.RemoveLabels) > 0 {
			for k := range instructions.RemoveLabels {
				if _, ok := builder.Labels[k]; ok {
					delete(builder.Labels, k)
				}
			}
		}

		if len(instructions.RemoveEnvs) > 0 &&
			len(builder.Env) > 0 {
			var newEnv []string
			for _, envPair := range builder.Env {
				envParts := strings.SplitN(envPair, "=", 2)
				if len(envParts) > 0 && envParts[0] != "" {
					if _, ok := instructions.RemoveEnvs[envParts[0]]; !ok {
						newEnv = append(newEnv, envPair)
					}
				}
			}

			builder.Env = newEnv
		}
	}

	if sourceImage != "" {
		builder.Labels[consts.DSLabelSourceImage] = sourceImage
	}

	if imageInfo != nil && imageInfo.ID != "" {
		builder.Labels[consts.DSLabelSourceImageID] = imageInfo.ID
	}

	builder.BuildOptions.OutputStream = &builder.BuildLog

	dataTar := filepath.Join(artifactLocation, "files.tar")
	builder.TarData = fsutil.IsRegularFile(dataTar)
	if builder.TarData {
		builder.HasData = true
	} else {
		dataDir := filepath.Join(artifactLocation, "files")
		builder.HasData = fsutil.IsDir(dataDir)
	}

	return builder, nil
}

// Build creates a new container image
func (b *SlimImageBuilder) Build() error {
	if err := b.GenerateDockerfile(); err != nil {
		return err
	}

	err := b.APIClient.BuildImage(b.BuildOptions)
	if err != nil {
		return err
	}

	for _, fullTag := range b.AdditionalTags {
		fullTag := strings.TrimSpace(fullTag)
		if len(fullTag) == 0 {
			log.Debug("SlimImageBuilder.Build: Skipping empty tag")
			continue
		}

		var options docker.TagImageOptions
		parts := strings.Split(fullTag, ":")
		if len(parts) > 2 {
			log.Debugf("SlimImageBuilder.Build: Skipping malformed tag - '%s'", fullTag)
			continue
		}

		if len(parts) == 2 {
			options.Repo = parts[0]
			options.Tag = parts[1]
		} else {
			options.Repo = parts[0]
		}

		targetImage := b.BuildOptions.Name
		if err := b.APIClient.TagImage(targetImage, options); err != nil {
			//not failing on tagging errors
			log.Debugf("SlimImageBuilder.Build: Error tagging image '%s' with tag - '%s' (error - %v)", targetImage, fullTag, err)
		}
	}

	return nil
}

// GenerateDockerfile creates a Dockerfile file
func (b *SlimImageBuilder) GenerateDockerfile() error {
	return dockerfile.GenerateFromInfo(b.BuildOptions.ContextDir,
		b.Volumes,
		b.WorkingDir,
		b.Env,
		b.Labels,
		b.User,
		b.ExposedPorts,
		b.Entrypoint,
		b.Cmd,
		b.HasData,
		b.TarData)
}
