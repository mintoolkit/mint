package internalbuilder

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
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
	Name                   = "internal.container.build.engine"
	DefaultAppDir          = "/opt/app"
	DefaultOutputImageName = "mint-built-image:latest"
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
	if len(options.Tags) == 0 {
		options.Tags = append(options.Tags, DefaultOutputImageName)
	}

	fileSourceLayerIndex := -1
	for i, layerInfo := range options.Layers {
		if layerInfo.Source == "" || !fsutil.Exists(layerInfo.Source) {
			continue
		}

		if layerInfo.Type == imagebuilder.FileSource {
			if !fsutil.IsRegularFile(layerInfo.Source) {
				continue
			}
		}

		if layerInfo.EntrypointLayer {
			fileSourceLayerIndex = i
		}
	}

	if options.From == "" &&
		options.FromTar == "" &&
		(options.ImageConfig == nil && fileSourceLayerIndex < 0) &&
		(options.ImageConfig != nil &&
			len(options.ImageConfig.Config.Entrypoint) == 0 &&
			len(options.ImageConfig.Config.Cmd) == 0) {
		return nil, fmt.Errorf("missing startup info - set options.From, options.FromTar or options.ImageConfig")
	}

	if len(options.Layers) == 0 {
		return nil, fmt.Errorf("no layers")
	}

	if len(options.Layers) > 255 {
		return nil, fmt.Errorf("too many layers")
	}

	var err error
	var img v1.Image
	var baseImageOS string
	var baseImageArch string
	var baseIsScratch bool
	if options.From == "" && options.FromTar == "" {
		img = empty.Image
		baseIsScratch = true
	} else {
		if options.FromTar != "" {
			if !fsutil.Exists(options.FromTar) || !fsutil.IsTarFile(options.FromTar) {
				return nil, fmt.Errorf("bad base image tar reference - %s", options.FromTar)
			}

			img, err = tarball.ImageFromPath(options.FromTar, nil)
			if err != nil {
				log.WithError(err).Error("tarball.ImageFromPath")
				return nil, err
			}
		} else {
			ref, err := name.ParseReference(options.From)
			if err != nil {
				log.WithError(err).Error("name.ParseReference")
				return nil, err
			}

			//TODO/FUTURE: add other image source options (not just local Docker daemon)
			//TODO/ASAP: need to pass the 'daemon' client otherwise it'll fail if the default client isn't enough
			img, err = daemon.Image(ref)
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
		}

		cf, err := img.ConfigFile()
		if err != nil {
			log.WithError(err).Error("v1.Image.ConfigFile")
			return nil, err
		}

		baseImageArch = cf.Architecture
		baseImageOS = cf.OS
	}

	if baseImageArch == "" {
		baseImageArch = runtime.GOARCH
	}

	if baseImageOS == "" {
		baseImageOS = "linux"
	}

	var newLayers []mutate.Addendum
	for i, layerInfo := range options.Layers {
		log.Debugf("DefaultSimpleBuilder.Build: [%d] create image layer (type=%v source=%s)",
			i, layerInfo.Type, layerInfo.Source)

		if layerInfo.Source == "" {
			return nil, fmt.Errorf("empty image layer data source")
		}

		if !fsutil.Exists(layerInfo.Source) {
			return nil, fmt.Errorf("image layer data source path does not exist - %s", layerInfo.Source)
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

			var layerFilePath string
			if layerInfo.Params != nil && layerInfo.Params.TargetPath != "" {
				layerFilePath = layerInfo.Params.TargetPath
			}

			if layerFilePath == "" {
				layerFilePath = path.Join(DefaultAppDir, filepath.Base(layerInfo.Source))
			}

			add := mutate.Addendum{
				Layer: layer,
			}

			if !options.HideBuildHistory {
				add.History = v1.History{
					Created:   v1.Time{Time: time.Now()},
					CreatedBy: fmt.Sprintf("COPY %s %s", layerInfo.Source, layerFilePath),
				}
			}

			newLayers = append(newLayers, add)
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

			add := mutate.Addendum{
				Layer: layer,
			}

			if !options.HideBuildHistory {
				add.History = v1.History{
					Created:   v1.Time{Time: time.Now()},
					CreatedBy: fmt.Sprintf("ADD %s /", layerInfo.Source),
				}
			}

			newLayers = append(newLayers, add)
		case imagebuilder.DirSource:
			if !fsutil.IsDir(layerInfo.Source) {
				return nil, fmt.Errorf("image layer data source path is not a directory - %s", layerInfo.Source)
			}

			layer, err := layerFromDir(layerInfo)
			if err != nil {
				return nil, err
			}

			layerBasePath := "/"
			if layerInfo.Params != nil && layerInfo.Params.TargetPath != "" {
				layerBasePath = layerInfo.Params.TargetPath
			}

			add := mutate.Addendum{
				Layer: layer,
			}

			if !options.HideBuildHistory {
				add.History = v1.History{
					Created:   v1.Time{Time: time.Now()},
					CreatedBy: fmt.Sprintf("COPY %s %s", layerInfo.Source, layerBasePath),
				}
			}

			newLayers = append(newLayers, add)
		default:
			return nil, fmt.Errorf("unknown image data source - %v", layerInfo.Source)
		}
	}

	log.Debug("DefaultSimpleBuilder.Build: adding new layers to image")
	newImg, err := mutate.Append(img, newLayers...)
	if err != nil {
		return nil, err
	}

	tag, err := name.NewTag(options.Tags[0])
	if err != nil {
		return nil, err
	}

	otherTags := options.Tags[1:]

	newImageConfig, err := newImg.ConfigFile()
	if err != nil {
		log.WithError(err).Error("newImg.ConfigFile")
		return nil, err
	}

	newImageConfig = newImageConfig.DeepCopy()
	if options.ImageConfig != nil {
		s2t := func(val string) v1.Time {
			ptime, err := time.Parse(time.RFC3339, val)
			if err != nil {
				log.WithError(err).Error("time.Parse")
				return v1.Time{Time: time.Now()}
			}
			return v1.Time{Time: ptime}
		}

		if len(options.ImageConfig.History) > 0 {
			var history []v1.History
			for _, h := range options.ImageConfig.History {
				history = append(history, v1.History{
					Created:    s2t(h.Created),
					CreatedBy:  h.CreatedBy,
					Comment:    h.Comment,
					Author:     h.Author,
					EmptyLayer: h.EmptyLayer,
				})
			}

			newImageConfig.History = history
		}

		for _, h := range options.ImageConfig.AddHistory {
			newImageConfig.History = append(newImageConfig.History, v1.History{
				Created:    s2t(h.Created),
				CreatedBy:  h.CreatedBy,
				Comment:    h.Comment,
				Author:     h.Author,
				EmptyLayer: h.EmptyLayer,
			})
		}
	}

	vaupdate := func(current *[]string, next []string) {
		if len(next) != 0 {
			*current = next
		}
	}

	vbupdate := func(current *bool, next bool) {
		//ok to have this simple update logic here
		if next {
			*current = next
		}
	}

	vupdate := func(current *string, next string) {
		if next != "" {
			*current = next
		}
	}

	vset := func(current *string, val string) {
		if *current == "" {
			*current = val
		}
	}

	configHist := func(inst string) v1.History {
		return v1.History{
			Created:    v1.Time{Time: time.Now()},
			CreatedBy:  inst,
			Comment:    "mintoolkit",
			EmptyLayer: true,
		}
	}

	var newConfigHistory []v1.History
	if options.ImageConfig != nil {
		switch options.ImageConfig.Architecture {
		case "", "arm64", "amd64":
		default:
			log.Errorf("bad architecture value - %s", options.ImageConfig.Architecture)
			return nil, fmt.Errorf("bad architecture value - %s", options.ImageConfig.Architecture)
		}

		vupdate(&newImageConfig.Config.User, options.ImageConfig.Config.User)
		if !options.HideBuildHistory && options.ImageConfig.Config.User != "" {
			newConfigHistory = append(newConfigHistory, configHist(
				fmt.Sprintf("USER %s", options.ImageConfig.Config.User),
			))
		}

		vupdate(&newImageConfig.Config.WorkingDir, options.ImageConfig.Config.WorkingDir)
		if !options.HideBuildHistory && options.ImageConfig.Config.WorkingDir != "" {
			newConfigHistory = append(newConfigHistory, configHist(
				fmt.Sprintf("WORKDIR %s", options.ImageConfig.Config.WorkingDir),
			))
		}

		vaupdate(&newImageConfig.Config.Entrypoint, options.ImageConfig.Config.Entrypoint)
		if !options.HideBuildHistory && options.ImageConfig.Config.Entrypoint != nil {
			var value string
			if options.ImageConfig.Config.IsShellEntrypoint {
				value = strings.Join(options.ImageConfig.Config.Entrypoint, " ")
			} else {
				value = fmt.Sprintf("[\"%s\"]",
					strings.Join(options.ImageConfig.Config.Entrypoint, `", "`))
			}
			newConfigHistory = append(newConfigHistory, configHist(
				fmt.Sprintf("ENTRYPOINT %s", value),
			))
		}

		vaupdate(&newImageConfig.Config.Cmd, options.ImageConfig.Config.Cmd)
		if !options.HideBuildHistory && options.ImageConfig.Config.Cmd != nil {
			var value string
			if options.ImageConfig.Config.IsShellCmd {
				value = strings.Join(options.ImageConfig.Config.Cmd, " ")
			} else {
				value = fmt.Sprintf("[\"%s\"]",
					strings.Join(options.ImageConfig.Config.Cmd, `", "`))
			}
			newConfigHistory = append(newConfigHistory, configHist(
				fmt.Sprintf("CMD %s", value),
			))
		}

		vupdate(&newImageConfig.Config.StopSignal, options.ImageConfig.Config.StopSignal)
		vbupdate(&newImageConfig.Config.ArgsEscaped, options.ImageConfig.Config.ArgsEscaped)
		vbupdate(&newImageConfig.Config.AttachStderr, options.ImageConfig.Config.AttachStderr)
		vbupdate(&newImageConfig.Config.AttachStdin, options.ImageConfig.Config.AttachStdin)
		vbupdate(&newImageConfig.Config.AttachStdout, options.ImageConfig.Config.AttachStdout)
		vupdate(&newImageConfig.Config.Domainname, options.ImageConfig.Config.Domainname)
		vupdate(&newImageConfig.Config.Hostname, options.ImageConfig.Config.Hostname)
		vupdate(&newImageConfig.Config.Image, options.ImageConfig.Config.Image)
		vaupdate(&newImageConfig.Config.OnBuild, options.ImageConfig.Config.OnBuild)
		vbupdate(&newImageConfig.Config.OpenStdin, options.ImageConfig.Config.OpenStdin)
		vbupdate(&newImageConfig.Config.StdinOnce, options.ImageConfig.Config.StdinOnce)
		vbupdate(&newImageConfig.Config.Tty, options.ImageConfig.Config.Tty)
		vbupdate(&newImageConfig.Config.NetworkDisabled, options.ImageConfig.Config.NetworkDisabled)
		vupdate(&newImageConfig.Config.MacAddress, options.ImageConfig.Config.MacAddress)
		vaupdate(&newImageConfig.Config.Shell, options.ImageConfig.Config.Shell)

		if newImageConfig.Config.ExposedPorts == nil {
			newImageConfig.Config.ExposedPorts = map[string]struct{}{}
		}

		if len(options.ImageConfig.Config.ExposedPorts) > 0 {
			newImageConfig.Config.ExposedPorts = options.ImageConfig.Config.ExposedPorts
			if !options.HideBuildHistory {
				for k := range options.ImageConfig.Config.ExposedPorts {
					newConfigHistory = append(newConfigHistory, configHist(
						fmt.Sprintf("EXPOSE %s", k),
					))
				}
			}
		}

		if len(options.ImageConfig.Config.AddExposedPorts) > 0 {
			for k, v := range options.ImageConfig.Config.AddExposedPorts {
				newImageConfig.Config.ExposedPorts[k] = v
			}
		}

		if len(options.ImageConfig.Config.RemoveExposedPorts) > 0 {
			for _, v := range options.ImageConfig.Config.RemoveExposedPorts {
				if _, found := newImageConfig.Config.ExposedPorts[v]; found {
					delete(newImageConfig.Config.ExposedPorts, v)
				}
			}
		}

		if options.ImageConfig.Config.Env != nil {
			//an empty Env will clear all env vars,
			//but a nil Env will leave the existing env vars as-is
			newImageConfig.Config.Env = options.ImageConfig.Config.Env
			if !options.HideBuildHistory {
				for _, v := range options.ImageConfig.Config.Env {
					newConfigHistory = append(newConfigHistory, configHist(
						fmt.Sprintf("ENV %s", v),
					))
				}
			}
		}

		for _, v := range options.ImageConfig.Config.AddEnv {
			newImageConfig.Config.Env = append(newImageConfig.Config.Env, v)
		}

		if newImageConfig.Config.Volumes == nil {
			newImageConfig.Config.Volumes = map[string]struct{}{}
		}

		if options.ImageConfig.Config.Volumes != nil {
			//an empty Volumes will clear all existing volume records,
			//but a nil Volumes will leave the existing volumes as-is
			newImageConfig.Config.Volumes = options.ImageConfig.Config.Volumes
			if !options.HideBuildHistory {
				for k := range options.ImageConfig.Config.Volumes {
					newConfigHistory = append(newConfigHistory, configHist(
						fmt.Sprintf("VOLUME %s", k),
					))
				}
			}
		}

		if len(options.ImageConfig.Config.AddVolumes) > 0 {
			for k, v := range options.ImageConfig.Config.AddVolumes {
				newImageConfig.Config.Volumes[k] = v
			}
		}

		if newImageConfig.Config.Labels == nil {
			newImageConfig.Config.Labels = map[string]string{}
		}

		if options.ImageConfig.Config.Labels != nil {
			//an empty Labels will clear all existing label records,
			//but a nil Labels will leave the existing labels as-is
			newImageConfig.Config.Labels = options.ImageConfig.Config.Labels
			if !options.HideBuildHistory {
				for k, v := range options.ImageConfig.Config.Labels {
					newConfigHistory = append(newConfigHistory, configHist(
						fmt.Sprintf("LABEL %s=%s", k, v),
					))
				}
			}
		}

		if len(options.ImageConfig.Config.AddLabels) > 0 {
			for k, v := range options.ImageConfig.Config.AddLabels {
				newImageConfig.Config.Labels[k] = v
			}
		}

		if options.ImageConfig.Config.Healthcheck != nil {
			newImageConfig.Config.Healthcheck = &v1.HealthConfig{
				Test:        options.ImageConfig.Config.Healthcheck.Test,
				Interval:    options.ImageConfig.Config.Healthcheck.Interval,
				Timeout:     options.ImageConfig.Config.Healthcheck.Timeout,
				StartPeriod: options.ImageConfig.Config.Healthcheck.StartPeriod,
				Retries:     options.ImageConfig.Config.Healthcheck.Retries,
			}
			//todo: add the HEALTHCHECK instruction history
		}

		newImageConfig.Created = v1.Time{Time: time.Now()}
		if !options.ImageConfig.Created.IsZero() {
			newImageConfig.Created = v1.Time{Time: options.ImageConfig.Created}
		}

		vupdate(&newImageConfig.Author, options.ImageConfig.Author)
		vset(&newImageConfig.Author, "mintoolkit")

		vupdate(&newImageConfig.Architecture, options.ImageConfig.Architecture)
		vset(&newImageConfig.Architecture, baseImageArch)

		vupdate(&newImageConfig.OS, options.ImageConfig.OS)
		vset(&newImageConfig.OS, baseImageOS)

		vupdate(&newImageConfig.OSVersion, options.ImageConfig.OSVersion)
		vaupdate(&newImageConfig.OSFeatures, options.ImageConfig.OSFeatures)
		vupdate(&newImageConfig.Variant, options.ImageConfig.Variant)
		vupdate(&newImageConfig.Container, options.ImageConfig.Container)
		vupdate(&newImageConfig.DockerVersion, options.ImageConfig.DockerVersion)
	} else {
		// a minor optimization when creating a simple single binary image from scratch
		// without an explicitly configured options.ImageConfig passed in
		if fileSourceLayerIndex > -1 {
			layerInfo := options.Layers[fileSourceLayerIndex]

			var layerFilePath string
			if layerInfo.Params != nil && layerInfo.Params.TargetPath != "" {
				layerFilePath = layerInfo.Params.TargetPath
			}

			if layerFilePath == "" {
				layerFilePath = path.Join(DefaultAppDir, filepath.Base(layerInfo.Source))
			}

			layerFileDir := filepath.Dir(layerFilePath)

			newImageConfig.Created = v1.Time{Time: time.Now()}
			newImageConfig.Author = "mintoolkit"
			if newImageConfig.Architecture == "" {
				newImageConfig.Architecture = baseImageArch
			}
			if newImageConfig.OS == "" {
				newImageConfig.OS = baseImageOS
			}

			newImageConfig.Config.Entrypoint = []string{layerFilePath}
			if !options.HideBuildHistory {
				newConfigHistory = append(newConfigHistory, configHist(
					fmt.Sprintf(`ENTRYPOINT ["%s"]`, layerFilePath),
				))
			}

			newImageConfig.Config.WorkingDir = layerFileDir

			if !baseIsScratch && layerInfo.ResetCmd {
				newImageConfig.Config.Cmd = []string{}
				//when you set entrypoint you reset the old cmd anyways...
				if !options.HideBuildHistory {
					newConfigHistory = append(newConfigHistory, configHist(
						"CMD []",
					))
				}
			}
		} else {
			return nil, fmt.Errorf("missing startup config info")
		}
	}

	if !options.HideBuildHistory && len(newConfigHistory) > 0 {
		newImageConfig.History = append(newImageConfig.History, newConfigHistory...)
	}

	log.Debug("DefaultSimpleBuilder.Build: update new image config metadata")
	newImg, err = mutate.ConfigFile(newImg, newImageConfig)
	if err != nil {
		log.WithError(err).Error("mutate.ConfigFile")
		return nil, err
	}

	if options.OutputImageTar != "" {
		if err := tarball.WriteToFile(options.OutputImageTar, tag, newImg); err != nil {
			log.WithError(err).Errorf("tarball.WriteToFile(%s, %s)", options.OutputImageTar, options.Tags[0])
			return nil, err
		}
	}

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
		layerFilePath = path.Join(DefaultAppDir, filepath.Base(input.Source))
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
		//need to make sure if it's an executable we have the right perms even if the source file exe bits are not set
		Mode: 0755, //int64(finfo.Mode()), //todo/later: don't assume it's an executable file (check if it's an exe or a script)
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
