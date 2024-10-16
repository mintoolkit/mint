package internalbuilder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

const (
	Name = "internal.container.build.engine"
)

// Engine is the default simple build engine
type Engine struct {
	ShowBuildLogs  bool
	PushToDaemon   bool
	PushToRegistry bool
}

// New creates new Engine instances
func New(
	showBuildLogs bool,
	pushToDaemon bool,
	pushToRegistry bool) (*Engine, error) {

	engine := &Engine{
		ShowBuildLogs:  showBuildLogs,
		PushToDaemon:   pushToDaemon,
		PushToRegistry: pushToRegistry,
	}

	return engine, nil
}

func (ref *Engine) Name() string {
	return Name
}

func (ref *Engine) Build(options imagebuilder.SimpleBuildOptions) (*imagebuilder.ImageResult, error) {
	if options.From == "" && options.ImageConfig == nil {
		return nil, fmt.Errorf("missing image config metadata - set options.From or options.ImageConfig")
	}

	if options.From == "" &&
		options.ImageConfig != nil &&
		len(options.ImageConfig.Config.Entrypoint) == 0 &&
		len(options.ImageConfig.Config.Cmd) == 0 {
		return nil, fmt.Errorf("missing startup info")
	}

	if len(options.Layers) == 0 {
		return nil, fmt.Errorf("no layers")
	}

	if len(options.Layers) > 255 {
		return nil, fmt.Errorf("too many layers")
	}

	var img v1.Image
	var baseImageOS string
	var baseImageArch string
	if options.From == "" {
		//same as FROM scratch
		img = empty.Image
	} else {
		ref, err := name.ParseReference(options.From)
		if err != nil {
			log.WithError(err).Error("name.ParseReference")
			return nil, err
		}

		//TODO/FUTURE: add other image source options (not just local Docker daemon)
		//TODO/ASAP: need to pass the 'daemon' client otherwise it'll fail if the default client isn't enough
		img, err := daemon.Image(ref)
		if err != nil {
			log.WithError(err).Debugf("daemon.Image(%s)", options.From)
			//return nil, err
			//TODO: have a flag to control the 'pull' behavior (also need to consider auth)
			//try to pull...
			img, err = remote.Image(ref)
			if err != nil {
				log.WithError(err).Errorf("remote.Image(%s)", options.From)
				return nil, err
			}
		}

		cf, err := img.ConfigFile()
		if err != nil {
			log.WithError(err).Error("v1.Image.ConfigFile")
			return nil, err
		}

		baseImageArch = cf.Architecture
		baseImageOS = cf.OS
	}

	if options.ImageConfig != nil {
		switch options.ImageConfig.Architecture {
		case "":
			options.ImageConfig.Architecture = baseImageArch
			if options.ImageConfig.Architecture == "" {
				options.ImageConfig.Architecture = "amd64"
			}
		case "arm64", "amd64":
		default:
			return nil, fmt.Errorf("bad architecture value")
		}

		imgRunConfig := v1.Config{
			User:            options.ImageConfig.Config.User,
			ExposedPorts:    options.ImageConfig.Config.ExposedPorts,
			Env:             options.ImageConfig.Config.Env,
			Entrypoint:      options.ImageConfig.Config.Entrypoint,
			Cmd:             options.ImageConfig.Config.Cmd,
			Volumes:         options.ImageConfig.Config.Volumes,
			WorkingDir:      options.ImageConfig.Config.WorkingDir,
			Labels:          options.ImageConfig.Config.Labels,
			StopSignal:      options.ImageConfig.Config.StopSignal,
			ArgsEscaped:     options.ImageConfig.Config.ArgsEscaped,
			AttachStderr:    options.ImageConfig.Config.AttachStderr,
			AttachStdin:     options.ImageConfig.Config.AttachStdin,
			AttachStdout:    options.ImageConfig.Config.AttachStdout,
			Domainname:      options.ImageConfig.Config.Domainname,
			Hostname:        options.ImageConfig.Config.Hostname,
			Image:           options.ImageConfig.Config.Image,
			OnBuild:         options.ImageConfig.Config.OnBuild,
			OpenStdin:       options.ImageConfig.Config.OpenStdin,
			StdinOnce:       options.ImageConfig.Config.StdinOnce,
			Tty:             options.ImageConfig.Config.Tty,
			NetworkDisabled: options.ImageConfig.Config.NetworkDisabled,
			MacAddress:      options.ImageConfig.Config.MacAddress,
			Shell:           options.ImageConfig.Config.Shell,
		}

		if options.ImageConfig.Config.Healthcheck != nil {
			imgRunConfig.Healthcheck = &v1.HealthConfig{
				Test:        options.ImageConfig.Config.Healthcheck.Test,
				Interval:    options.ImageConfig.Config.Healthcheck.Interval,
				Timeout:     options.ImageConfig.Config.Healthcheck.Timeout,
				StartPeriod: options.ImageConfig.Config.Healthcheck.StartPeriod,
				Retries:     options.ImageConfig.Config.Healthcheck.Retries,
			}
		}

		imgConfig := &v1.ConfigFile{
			Created:      v1.Time{Time: time.Now()},
			Author:       options.ImageConfig.Author,
			Architecture: options.ImageConfig.Architecture,
			OS:           options.ImageConfig.OS,
			OSVersion:    options.ImageConfig.OSVersion,
			OSFeatures:   options.ImageConfig.OSFeatures,
			Variant:      options.ImageConfig.Variant,
			Config:       imgRunConfig,
			//History - not setting for now (actual history needs to match the added layers)
			Container:     options.ImageConfig.Container,
			DockerVersion: options.ImageConfig.DockerVersion,
		}

		if imgConfig.OS == "" {
			imgConfig.OS = baseImageOS
			if imgConfig.OS == "" {
				imgConfig.OS = "linux"
			}
		}

		if imgConfig.Author == "" {
			imgConfig.Author = "mintoolkit"
		}

		if !options.ImageConfig.Created.IsZero() {
			imgConfig.Created = v1.Time{Time: options.ImageConfig.Created}
		}

		log.Debug("DefaultSimpleBuilder.Build: config image")

		var err error
		img, err = mutate.ConfigFile(img, imgConfig)
		if err != nil {
			return nil, err
		}
	}

	var layersToAdd []v1.Layer

	for i, layerInfo := range options.Layers {
		log.Debugf("DefaultSimpleBuilder.Build: [%d] create image layer (type=%v source=%s)",
			i, layerInfo.Type, layerInfo.Source)

		if layerInfo.Source == "" {
			return nil, fmt.Errorf("empty image layer data source")
		}

		if !fsutil.Exists(layerInfo.Source) {
			return nil, fmt.Errorf("image layer data source path doesnt exist - %s", layerInfo.Source)
		}

		switch layerInfo.Type {
		case imagebuilder.FileSource:
			if !fsutil.IsRegularFile(layerInfo.Source) {
				return nil, fmt.Errorf("image layer data source path is not a file - %s", layerInfo.Source)
			}

			layer, err := layerFromFile(layerInfo)
			if err != nil {
				return nil, err
			}

			layersToAdd = append(layersToAdd, layer)
		case imagebuilder.TarSource:
			if !fsutil.IsRegularFile(layerInfo.Source) {
				return nil, fmt.Errorf("image layer data source path is not a file - %s", layerInfo.Source)
			}

			if !fsutil.IsTarFile(layerInfo.Source) {
				return nil, fmt.Errorf("image layer data source path is not a tar file - %s", layerInfo.Source)
			}

			layer, err := layerFromTar(layerInfo)
			if err != nil {
				return nil, err
			}

			layersToAdd = append(layersToAdd, layer)
		case imagebuilder.DirSource:
			if !fsutil.IsDir(layerInfo.Source) {
				return nil, fmt.Errorf("image layer data source path is not a directory - %s", layerInfo.Source)
			}

			layer, err := layerFromDir(layerInfo)
			if err != nil {
				return nil, err
			}

			layersToAdd = append(layersToAdd, layer)
		default:
			return nil, fmt.Errorf("unknown image data source - %v", layerInfo.Source)
		}
	}

	log.Debug("DefaultSimpleBuilder.Build: adding layers to image")
	newImg, err := mutate.AppendLayers(img, layersToAdd...)
	if err != nil {
		return nil, err
	}

	if len(options.Tags) == 0 {
		return nil, fmt.Errorf("missing tags")
	}

	tag, err := name.NewTag(options.Tags[0])
	if err != nil {
		return nil, err
	}

	otherTags := options.Tags[1:]

	if ref.PushToDaemon {
		log.Debug("DefaultSimpleBuilder.Build: saving image to Docker")
		imageLoadResponseStr, err := daemon.Write(tag, newImg)
		if err != nil {
			return nil, err
		}

		log.Debugf("DefaultSimpleBuilder.Build: pushed image to daemon - %s", imageLoadResponseStr)
		if ref.ShowBuildLogs {
			//TBD (need execution context to display the build logs)
		}

		if len(otherTags) > 0 {
			log.Debug("DefaultSimpleBuilder.Build: adding other tags")

			for _, tagName := range otherTags {
				ntag, err := name.NewTag(tagName)
				if err != nil {
					log.Errorf("DefaultSimpleBuilder.Build: error creating tag: %v", err)
					continue
				}

				if err := daemon.Tag(tag, ntag); err != nil {
					log.Errorf("DefaultSimpleBuilder.Build: error tagging: %v", err)
				}
			}
		}
	}

	if ref.PushToRegistry {
		//TBD
	}

	id, _ := newImg.ConfigName()
	digest, _ := newImg.Digest()
	result := &imagebuilder.ImageResult{
		Name:      options.Tags[0],
		OtherTags: otherTags,
		ID:        fmt.Sprintf("%s:%s", id.Algorithm, id.Hex),
		Digest:    fmt.Sprintf("%s:%s", digest.Algorithm, digest.Hex),
	}

	return result, nil
}

func layerFromTar(input imagebuilder.LayerDataInfo) (v1.Layer, error) {
	if !fsutil.Exists(input.Source) ||
		!fsutil.IsRegularFile(input.Source) {
		return nil, fmt.Errorf("bad input data")
	}

	return tarball.LayerFromFile(input.Source)
}

func layerFromFile(input imagebuilder.LayerDataInfo) (v1.Layer, error) {
	if !fsutil.Exists(input.Source) ||
		!fsutil.IsRegularFile(input.Source) {
		return nil, fmt.Errorf("bad input data")
	}

	f, err := os.Open(input.Source)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	finfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	var layerFilePath string
	if input.Params != nil && input.Params.TargetPath != "" {
		layerFilePath = input.Params.TargetPath
	}

	if layerFilePath == "" {
		layerFilePath = path.Join("/opt/app", filepath.Base(input.Source))
	}

	layerFilePath = strings.TrimLeft(layerFilePath, "/")
	layerFileDir := filepath.Dir(layerFilePath)
	layerFileDirParts := strings.Split(layerFileDir, "/")

	var dirPrefix string
	for _, part := range layerFileDirParts {
		var currentDirPath string
		if dirPrefix != "" {
			dirPrefix = path.Join(dirPrefix, part)
		} else {
			dirPrefix = part
		}

		currentDirPath = path.Join(dirPrefix, "/")
		if err := tw.WriteHeader(
			&tar.Header{
				Name:     currentDirPath,
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}); err != nil {
			return nil, fmt.Errorf("failed to write tar header for dir: %w", err)
		}
	}

	hdr := &tar.Header{
		Name: layerFilePath,
		Mode: int64(finfo.Mode()),
		Size: finfo.Size(),
	}

	if finfo.Mode().IsRegular() {
		hdr.Typeflag = tar.TypeReg
	} else {
		return nil, fmt.Errorf("not implemented archiving file type %s (%s)", finfo.Mode(), layerFilePath)
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("failed to write tar header for file(%s): %w", layerFilePath, err)
	}

	if _, err := io.Copy(tw, f); err != nil {
		return nil, fmt.Errorf("failed to read file(%s) into the tar: %w", layerFilePath, err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finish tar: %w", err)
	}

	return tarball.LayerFromReader(&b)
}

func layerFromDir(input imagebuilder.LayerDataInfo) (v1.Layer, error) {
	if !fsutil.Exists(input.Source) ||
		!fsutil.IsDir(input.Source) {
		return nil, fmt.Errorf("bad input data")
	}

	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	layerBasePath := "/"
	if input.Params != nil && input.Params.TargetPath != "" {
		layerBasePath = input.Params.TargetPath
	}

	err := filepath.Walk(input.Source, func(fp string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(input.Source, fp)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		hdr := &tar.Header{
			Name: path.Join(layerBasePath, filepath.ToSlash(rel)),
			Mode: int64(info.Mode()),
		}

		if !info.IsDir() {
			hdr.Size = info.Size()
		}

		if info.Mode().IsDir() {
			hdr.Typeflag = tar.TypeDir
		} else if info.Mode().IsRegular() {
			hdr.Typeflag = tar.TypeReg
		} else {
			return fmt.Errorf("not implemented archiving file type %s (%s)", info.Mode(), rel)
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}
		if !info.IsDir() {
			f, err := os.Open(fp)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, f); err != nil {
				return fmt.Errorf("failed to read file into the tar: %w", err)
			}
			f.Close()
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan files: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finish tar: %w", err)
	}

	return tarball.LayerFromReader(&b)
}
