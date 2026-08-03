package dockercrtclient

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerutil"
	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

type Instance struct {
	pclient       *docker.Client
	showBuildLogs bool
	buildLog      bytes.Buffer
}

func New(providerClient *docker.Client) *Instance {
	return &Instance{
		pclient: providerClient,
	}
}

func NewBuilder(providerClient *docker.Client, showBuildLogs bool) *Instance {
	ref := New(providerClient)
	ref.showBuildLogs = showBuildLogs
	return ref
}

func (ref *Instance) BuildImage(options imagebuilder.DockerfileBuildOptions) error {
	if len(options.Labels) > 0 {
		//Docker has a limit on how long the labels can be
		labels := map[string]string{}
		for k, v := range options.Labels {
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
		options.Labels = labels
	}

	if options.BuildContext == "" {
		options.BuildContext = "."
	}

	//not using options.CacheTo in this image builder...
	buildOptions := docker.BuildImageOptions{
		Dockerfile: options.Dockerfile,
		Target:     options.Target,
		Name:       options.ImagePath,

		NetworkMode:    options.NetworkMode,
		ExtraHosts:     options.ExtraHosts,
		CacheFrom:      options.CacheFrom,
		Labels:         options.Labels,
		RmTmpContainer: true,
	}

	if len(options.Platforms) > 0 {
		buildOptions.Platform = strings.Join(options.Platforms, ",")
	}

	for _, nv := range options.BuildArgs {
		buildOptions.BuildArgs = append(buildOptions.BuildArgs, docker.BuildArg{Name: nv.Name, Value: nv.Value})
	}

	if strings.HasPrefix(options.BuildContext, "http://") ||
		strings.HasPrefix(options.BuildContext, "https://") {
		buildOptions.Remote = options.BuildContext
	} else {
		if exists := fsutil.DirExists(options.BuildContext); exists {
			buildOptions.ContextDir = options.BuildContext
			//Dockerfile path is expected to be relative to build context
			fullDockerfileName := filepath.Join(buildOptions.ContextDir, buildOptions.Dockerfile)
			if !fsutil.Exists(fullDockerfileName) || !fsutil.IsRegularFile(fullDockerfileName) {
				return fmt.Errorf("invalid dockerfile reference (%s) - %s", buildOptions.Dockerfile, fullDockerfileName)
			}
		} else {
			return imagebuilder.ErrInvalidContextDir
		}
	}

	if options.OutputStream != nil {
		buildOptions.OutputStream = options.OutputStream
	} else {
		ref.buildLog.Reset()
		buildOptions.OutputStream = &ref.buildLog
	}

	return ref.pclient.BuildImage(buildOptions)
}

func (ref *Instance) BuildOutputLog() string {
	return ref.buildLog.String()
}

func (ref *Instance) HasImage(imageRef string) (*crt.ImageIdentity, error) {
	pii, err := dockerutil.HasImage(ref.pclient, imageRef)
	if err != nil {
		if err == dockerutil.ErrNotFound {
			err = crt.ErrNotFound
		}

		return nil, err
	}
	ii := &crt.ImageIdentity{
		ID:           pii.ID,
		ShortTags:    pii.ShortTags,
		RepoTags:     pii.RepoTags,
		ShortDigests: pii.ShortDigests,
		RepoDigests:  pii.RepoDigests,
	}

	return ii, nil
}

func (ref *Instance) ListImagesAll() ([]crt.BasicImageInfo, error) {
	pimages, err := ref.pclient.ListImages(docker.ListImagesOptions{All: true})
	if err != nil {
		return nil, err
	}

	var imageList []crt.BasicImageInfo
	for _, r := range pimages {
		imageList = append(imageList, crt.BasicImageInfo{
			ID:          r.ID,
			Size:        r.Size,
			Created:     r.Created,
			ParentID:    r.ParentID,
			RepoTags:    r.RepoTags,
			RepoDigests: r.RepoDigests,
			Labels:      r.Labels,
		})
	}

	return imageList, nil
}

func (ref *Instance) ListImages(imageNameFilter string) (map[string]crt.BasicImageInfo, error) {
	pimages, err := dockerutil.ListImages(ref.pclient, imageNameFilter)
	if err != nil {
		return nil, err
	}

	images := map[string]crt.BasicImageInfo{}
	for k, v := range pimages {
		images[k] = crt.BasicImageInfo{
			ID:      v.ID,
			Size:    v.Size,
			Created: v.Created,
		}
	}

	return images, nil
}

func (ref *Instance) InspectImage(imageRef string) (*crt.ImageInfo, error) {
	pimage, err := ref.pclient.InspectImage(imageRef)
	if err != nil {
		if err == docker.ErrNoSuchImage {
			return nil, crt.ErrNotFound
		}

		return nil, err
	}

	result := &crt.ImageInfo{
		RuntimeName:    "docker",
		RuntimeVersion: pimage.DockerVersion,
		Created:        pimage.Created,
		ID:             pimage.ID,
		RepoTags:       pimage.RepoTags,
		RepoDigests:    pimage.RepoDigests,
		Size:           pimage.Size,
		VirtualSize:    pimage.VirtualSize,
		OS:             pimage.OS,
		Architecture:   pimage.Architecture,
		Author:         pimage.Author,
	}

	if pimage.Config != nil {
		result.Config = &crt.RunConfig{
			User:            pimage.Config.User,
			Env:             pimage.Config.Env,
			Entrypoint:      pimage.Config.Entrypoint,
			Cmd:             pimage.Config.Cmd,
			Volumes:         pimage.Config.Volumes,
			WorkingDir:      pimage.Config.WorkingDir,
			Labels:          pimage.Config.Labels,
			StopSignal:      pimage.Config.StopSignal,
			ArgsEscaped:     pimage.Config.ArgsEscaped,
			AttachStderr:    pimage.Config.AttachStderr,
			AttachStdin:     pimage.Config.AttachStdin,
			AttachStdout:    pimage.Config.AttachStdout,
			Domainname:      pimage.Config.Domainname,
			Hostname:        pimage.Config.Hostname,
			Image:           pimage.Config.Image,
			OnBuild:         pimage.Config.OnBuild,
			OpenStdin:       pimage.Config.OpenStdin,
			StdinOnce:       pimage.Config.StdinOnce,
			Tty:             pimage.Config.Tty,
			NetworkDisabled: pimage.Config.NetworkDisabled,
			MacAddress:      pimage.Config.MacAddress,
			StopTimeout:     &pimage.Config.StopTimeout,
			Shell:           pimage.Config.Shell,
		}

		if pimage.Config.ExposedPorts != nil {
			result.Config.ExposedPorts = map[string]struct{}{}
			for k, v := range pimage.Config.ExposedPorts {
				result.Config.ExposedPorts[string(k)] = v
			}
		}

		if pimage.Config.Healthcheck != nil {
			result.Config.Healthcheck = &crt.HealthConfig{
				Test:          pimage.Config.Healthcheck.Test,
				Interval:      pimage.Config.Healthcheck.Interval,
				Timeout:       pimage.Config.Healthcheck.Timeout,
				StartPeriod:   pimage.Config.Healthcheck.StartPeriod,
				StartInterval: pimage.Config.Healthcheck.StartInterval,
				Retries:       pimage.Config.Healthcheck.Retries,
			}
		}
	}

	return result, nil
}

func (ref *Instance) PullImage(opts crt.PullImageOptions, authConfig crt.AuthConfig) error {
	input := docker.PullImageOptions{
		Repository: opts.Repository,
		Tag:        opts.Tag,
	}

	if opts.OutputStream != nil {
		input.OutputStream = opts.OutputStream
	}

	var authConfiguration *docker.AuthConfiguration
	if authConfig != nil {
		var ok bool
		authConfiguration, ok = authConfig.(*docker.AuthConfiguration)
		if !ok {
			return fmt.Errorf("invalid *docker.AuthConfiguration")
		}
	}

	if authConfiguration == nil {
		authConfiguration = &docker.AuthConfiguration{}
	}

	return ref.pclient.PullImage(input, *authConfiguration)
}

func (ref *Instance) GetRegistryAuthConfig(account, secret, configPath, registry string) (crt.AuthConfig, error) {
	if account != "" || secret != "" {
		return &docker.AuthConfiguration{
			Username: account,
			Password: secret,
		}, nil
	}

	missingAuthConfigErr := fmt.Errorf("could not find an auth config for registry - %s", registry)
	if configPath != "" {
		dAuthConfigs, err := docker.NewAuthConfigurationsFromFile(configPath)
		if err != nil {
			log.WithError(err).Warnf(
				"dockercrtclient.Instance.GetRegistryAuthConfig('%s',secret,'%s','%s'): getDockerCredential - failed to acquire local docker config path",
				account, configPath, registry)
			return nil, err
		}
		r, found := dAuthConfigs.Configs[registry]
		if !found {
			return nil, missingAuthConfigErr
		}
		return &r, nil
	}

	cred, err := docker.NewAuthConfigurationsFromCredsHelpers(registry)
	if err != nil {
		log.WithError(err).Warnf(
			"dockercrtclient.Instance.GetRegistryAuthConfig('%s',secret,'%s','%s'): failed to acquire local docker credential helpers",
			account, configPath, registry)
		return nil, err
	}

	// could not find a credentials' helper, check auth configs
	if cred == nil {
		dConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
		if err != nil {
			log.WithError(err).Error("dockercrtclient.Instance.GetRegistryAuthConfig: docker.NewAuthConfigurationsFromDockerCfg err extracting docker auth configs")
			return nil, err
		}
		r, found := dConfigs.Configs[registry]
		if !found {
			return nil, missingAuthConfigErr
		}
		cred = &r
	}

	log.Tracef("dockercrtclient.Instance.GetRegistryAuthConfig: loaded registry auth config %+v", cred)
	return cred, nil
}

func (ref *Instance) SaveImage(imageRef, localPath string, extract, removeOrig bool, inactivityTimeout int) error {
	err := dockerutil.SaveImage(ref.pclient, imageRef, localPath, extract, removeOrig, inactivityTimeout)
	if err != nil {
		if err == dockerutil.ErrBadParam {
			err = crt.ErrBadParam
		}
		return err
	}

	return nil
}

func (ref *Instance) LoadImage(localPath string, outputStream io.Writer) error {
	err := dockerutil.LoadImage(ref.pclient, localPath, nil, outputStream)
	if err != nil {
		if err == dockerutil.ErrBadParam {
			err = crt.ErrBadParam
		}
		return err
	}

	return nil
}

func (ref *Instance) GetImagesHistory(imageRef string) ([]crt.ImageHistory, error) {
	phistory, err := ref.pclient.ImageHistory(imageRef)
	if err != nil {
		return nil, err
	}

	var result []crt.ImageHistory
	for _, r := range phistory {
		result = append(result, crt.ImageHistory{
			ID:        r.ID,
			Created:   r.Created,
			CreatedBy: r.CreatedBy,
			Tags:      r.Tags,
			Size:      r.Size,
			Comment:   r.Comment,
		})
	}

	return result, nil
}
