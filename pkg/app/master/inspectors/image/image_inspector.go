package image

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/consts"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/docker/dockerfile/reverse"
	"github.com/mintoolkit/mint/pkg/util/errutil"
)

const (
	slimImageRepo          = "mint"
	appArmorProfileName    = "apparmor-profile"
	seccompProfileName     = "seccomp-profile"
	appArmorProfileNamePat = "%s-apparmor-profile"
	seccompProfileNamePat  = "%s-seccomp.json"
	// https                  = "https://"
	// http                   = "http://"
)

// Inspector is a container image inspector
type Inspector struct {
	ImageRef            string
	ArtifactLocation    string
	SlimImageRepo       string
	AppArmorProfileName string
	SeccompProfileName  string
	ImageInfo           *crt.ImageInfo         // *docker.Image
	ImageRecordInfo     crt.BasicImageInfo     // docker.APIImages
	APIClient           crt.InspectorAPIClient //*docker.Client
	//fatImageDockerInstructions []string
	DockerfileInfo *reverse.Dockerfile
}

// NewInspector creates a new container image inspector
func NewInspector(apiClient crt.InspectorAPIClient, imageRef string /*, artifactLocation string*/) (*Inspector, error) {
	inspector := &Inspector{
		ImageRef:            imageRef,
		SlimImageRepo:       slimImageRepo,
		AppArmorProfileName: appArmorProfileName,
		SeccompProfileName:  seccompProfileName,
		//ArtifactLocation:    artifactLocation,
		APIClient: apiClient,
	}

	return inspector, nil
}

// NoImage returns true if the target image doesn't exist
func (i *Inspector) NoImage() (bool, error) {
	//first, do a simple exact match lookup
	ii, err := i.APIClient.HasImage(i.ImageRef)
	if err == nil {
		log.Tracef("image.inspector.NoImage: ImageRef=%v ImageIdentity=%#v", i.ImageRef, ii)
		return false, nil
	}

	if err != crt.ErrNotFound {
		log.Errorf("image.inspector.NoImage: err=%v", err)
		return true, err
	}

	//second, try to find something close enough
	//handle the case where there's no tag in the target image reference
	//and there are no default 'latest' tag
	//this will return/save the first available tag
	if err == crt.ErrNotFound &&
		!strings.Contains(i.ImageRef, ":") {
		log.Debugf("image.inspector.NoImage: no default 'latest' tag / i.ImageRef='%s'", i.ImageRef)
		//check if there are any tags for the target image
		matches, err := i.APIClient.ListImages(i.ImageRef)
		if err != nil {
			log.Errorf("image.inspector.NoImage: err=%v", err)
			return true, err
		}

		log.Debugf("image.inspector.NoImage: matching image tag count - %d / i.ImageRef='%s'", len(matches), i.ImageRef)
		for ref, props := range matches {
			log.Debugf("image.inspector.NoImage: match.ref='%s' match.props=%#v", ref, props)
			i.ImageRef = ref
			return false, nil
		}
	}

	return true, nil
}

// Pull tries to download the target image
func (i *Inspector) Pull(showPullLog bool, dockerConfigPath, registryAccount, registrySecret, platform string) error {
	var pullLog bytes.Buffer
	var repo string
	var tag string
	if strings.Contains(i.ImageRef, ":") {
		parts := strings.SplitN(i.ImageRef, ":", 2)
		repo = parts[0]
		tag = parts[1]
	} else {
		repo = i.ImageRef
		tag = "latest"
	}

	input := crt.PullImageOptions{
		Repository: repo,
		Tag:        tag,
		Platform:   platform,
	}

	if showPullLog {
		input.OutputStream = &pullLog
	}

	var err error
	var authConfig crt.AuthConfig
	registry := crt.ExtractRegistry(repo)
	authConfig, err = i.APIClient.GetRegistryAuthConfig(registryAccount, registrySecret, dockerConfigPath, registry)
	if err != nil {
		log.WithError(err).Warnf("image.inspector.Pull: [repo='%s'/tag='%s'] failed to get registry credential for registry='%s'",
			repo, tag, registry)
		//warn, attempt pull anyway, needs to work for public registries
	}

	err = i.APIClient.PullImage(input, authConfig)
	if err != nil {
		log.WithError(err).Debugf("image.inspector.Pull: client.PullImage")
		return err
	}

	if showPullLog {
		fmt.Printf("pull logs ====================\n")
		fmt.Println(pullLog.String())
		fmt.Printf("end of pull logs =============\n")
	}

	return nil
}

// Inspect starts the target image inspection
func (i *Inspector) Inspect() error {
	var err error
	i.ImageInfo, err = i.APIClient.InspectImage(i.ImageRef)
	if err != nil {
		if err == crt.ErrNotFound {
			log.Infof("could not find target image - i.ImageRef='%s'", i.ImageRef)
		}
		return err
	}

	log.Tracef("image.Inspector.Inspect: ImageInfo=%#v", i.ImageInfo)

	imageList, err := i.APIClient.ListImagesAll()
	if err != nil {
		return err
	}

	log.Tracef("image.Inspector.Inspect: imageList.size=%v", len(imageList))
	for _, r := range imageList {
		log.Tracef("image.Inspector.Inspect: target=%v record=%#v", i.ImageInfo.ID, r)
		if strings.Contains(i.ImageInfo.ID, r.ID) {
			i.ImageRecordInfo = r
			break
		}
	}

	if i.ImageRecordInfo.ID == "" {
		log.Infof("could not find target image in the image list - i.ImageRef='%s' / i.ImageInfo.ID='%s'",
			i.ImageRef, i.ImageInfo.ID)
		return crt.ErrNotFound
	}

	return nil
}

func (i *Inspector) processImageName() {
	if len(i.ImageRecordInfo.RepoTags) > 0 {
		//try to find the repo/tag that matches the image ref (if it's not an image ID)
		//then pick the first available repo/tag if we can't
		imageName := i.ImageRecordInfo.RepoTags[0]
		for _, current := range i.ImageRecordInfo.RepoTags {
			if strings.HasPrefix(current, i.ImageRef) {
				imageName = current
				break
			}
		}

		if rtInfo := strings.Split(imageName, ":"); len(rtInfo) > 1 {
			if rtInfo[0] == "<none>" {
				rtInfo[0] = strings.TrimLeft(i.ImageRecordInfo.ID, "sha256:")[0:8]
			}
			i.SlimImageRepo = fmt.Sprintf("%s.slim", rtInfo[0])
			if nameParts := strings.Split(rtInfo[0], "/"); len(nameParts) > 1 {
				i.AppArmorProfileName = strings.Join(nameParts, "-")
				i.SeccompProfileName = strings.Join(nameParts, "-")
			} else {
				i.AppArmorProfileName = rtInfo[0]
				i.SeccompProfileName = rtInfo[0]
			}
			i.AppArmorProfileName = fmt.Sprintf(appArmorProfileNamePat, i.AppArmorProfileName)
			i.SeccompProfileName = fmt.Sprintf(seccompProfileNamePat, i.SeccompProfileName)
		}
	}
}

// ProcessCollectedData performs post-processing on the collected image data
func (i *Inspector) ProcessCollectedData() error {
	i.processImageName()

	var err error
	i.DockerfileInfo, err = reverse.DockerfileFromHistory(i.APIClient, i.ImageRef)
	if err != nil {
		return err
	}
	fatImageDockerfileLocation := filepath.Join(i.ArtifactLocation, consts.ReversedDockerfile)
	err = reverse.SaveDockerfileData(fatImageDockerfileLocation, i.DockerfileInfo.Lines)
	errutil.FailOn(err)
	//save the reversed Dockerfile with the old name too (tmp compat)
	fatImageDockerfileLocationOld := filepath.Join(i.ArtifactLocation, consts.ReversedDockerfileOldName)
	err = reverse.SaveDockerfileData(fatImageDockerfileLocationOld, i.DockerfileInfo.Lines)
	errutil.WarnOn(err)

	return nil
}

// ShowFatImageDockerInstructions prints the original target image Dockerfile instructions
func (i *Inspector) ShowFatImageDockerInstructions() {
	if i.DockerfileInfo != nil && i.DockerfileInfo.Lines != nil {
		fmt.Println("mint: Fat image - Dockerfile instructures: start ====")
		fmt.Println(strings.Join(i.DockerfileInfo.Lines, "\n"))
		fmt.Println("mint: Fat image - Dockerfile instructures: end ======")
	}
}
