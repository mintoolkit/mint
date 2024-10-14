package podmanutil

import (
	"context"
	"io"
	// "archive/tar"
	// "bufio"
	// "bytes"
	"errors"
	"fmt"
	// "io"
	"os"
	"path/filepath"
	"strings"
	// "time"

	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/storage"
	"github.com/docker/docker/pkg/archive"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

var (
	ErrBadParam = errors.New("bad parameter")
	ErrNotFound = errors.New("not found")
)

type BasicImageProps struct {
	ID      string
	Size    int64
	Created int64
}

type ImageIdentity struct {
	ID           string
	ShortTags    []string
	RepoTags     []string
	ShortDigests []string
	RepoDigests  []string
}

func ImageToIdentity(info *types.ImageInspectReport) *ImageIdentity {
	result := &ImageIdentity{
		ID:          info.ID,
		RepoTags:    info.RepoTags,
		RepoDigests: info.RepoDigests,
	}

	uniqueTags := map[string]struct{}{}
	for _, tag := range result.RepoTags {
		parts := strings.Split(tag, ":")
		if len(parts) == 2 {
			uniqueTags[parts[1]] = struct{}{}
		}
	}

	for k := range uniqueTags {
		result.ShortTags = append(result.ShortTags, k)
	}

	uniqueDigests := map[string]struct{}{}
	for _, digest := range result.RepoDigests {
		parts := strings.Split(digest, "@")
		if len(parts) == 2 {
			uniqueDigests[parts[1]] = struct{}{}
		}
	}

	for k := range uniqueDigests {
		result.ShortDigests = append(result.ShortDigests, k)
	}

	return result
}

func HasImage(client context.Context, imageRef string) (*ImageIdentity, error) {
	if imageRef == "" || imageRef == "." || imageRef == ".." {
		return nil, ErrBadParam
	}

	exists, err := images.Exists(client, imageRef, nil)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	imageInfo, err := images.GetImage(client, imageRef, nil)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return ImageToIdentity(imageInfo), nil
}

func LoadImage(client context.Context,
	locaTarFilePath string,
	inputStream io.Reader,
	outputStream io.Writer) error {
	if locaTarFilePath == "" && inputStream == nil {
		return ErrBadParam
	}

	if locaTarFilePath != "" {
		if !fsutil.Exists(locaTarFilePath) {
			return ErrBadParam
		}

		dfile, err := os.Open(locaTarFilePath)
		if err != nil {
			return err
		}

		defer dfile.Close()

		inputStream = dfile
	}

	report, err := images.Load(client, inputStream)
	if err != nil {
		log.Errorf("podmanutil.LoadImage: images.Load() error = %v", err)
		return err
	}

	log.Errorf("podmanutil.LoadImage: images.Load() report = %+v", report)
	//todo: if outputStream != nil write report to it (report.Names)
	return nil
}

func SaveImage(
	client context.Context,
	imageRef string,
	local string,
	extract bool,
	removeOrig bool,
	useDockerFormat bool,
	inactivityTimeout int) error {
	if local == "" {
		return ErrBadParam
	}

	//todo: 'pull' the image if it's not available locally yet
	exists, err := images.Exists(client, imageRef, nil)
	if !exists {
		return fmt.Errorf("target image '%s' does not exist", imageRef)
	}

	dir := fsutil.FileDir(local)
	if !fsutil.DirExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	dfile, err := os.Create(local)
	if err != nil {
		return err
	}

	formatName := "oci"
	if useDockerFormat {
		formatName = "docker-archive"
	}
	options := &images.ExportOptions{
		Format: &formatName,
	}

	err = images.Export(client, []string{imageRef}, dfile, options)
	if err != nil {
		log.Errorf("podmanutil.SaveImage: images.Export() error = %v", err)
		dfile.Close()
		return err
	}

	dfile.Close()

	if extract {
		dstDir := filepath.Dir(local)
		arc := archive.NewDefaultArchiver()

		afile, err := os.Open(local)
		if err != nil {
			log.Errorf("dockerutil.SaveImage: os.Open error - %v", err)
			return err
		}

		tarOptions := &archive.TarOptions{
			NoLchown: true,
			//UIDMaps:  arc.IDMapping.UIDs(),
			//GIDMaps:  arc.IDMapping.GIDs(),

		}

		tarOptions.IDMap.UIDMaps = arc.IDMapping.UIDMaps
		tarOptions.IDMap.GIDMaps = arc.IDMapping.GIDMaps

		err = arc.Untar(afile, dstDir, tarOptions)
		if err != nil {
			log.Errorf("dockerutil.SaveImage: error unpacking tar - %v", err)
			afile.Close()
			return err
		}

		afile.Close()

		if removeOrig {
			os.Remove(local)
		}
	}

	return nil
}

func ListImages(client context.Context, imageNameFilter string) (map[string]BasicImageProps, error) {
	//needs extra testing...
	//limited 'reference' filtering (wildcard usable only in the image name,
	//not account or domain or can wildcard the domain/account)
	options := &images.ListOptions{}
	if imageNameFilter == "" {
		options.WithAll(true)
	} else {
		options.WithAll(false)
		options.WithFilters(map[string][]string{
			"reference": {imageNameFilter},
		})
	}

	imageList, err := images.List(client, options)
	if err != nil {
		log.Errorf("podmanutil.ListImages(%s): images.List() error = %v", imageNameFilter, err)
		return nil, err
	}

	log.Debugf("podmanutil.ListImages(%s): matching images - %+v", imageNameFilter, imageList)

	images := map[string]BasicImageProps{}
	for _, imageInfo := range imageList {
		for _, repo := range imageInfo.RepoTags {
			info := BasicImageProps{
				ID:      imageInfo.ID,
				Size:    imageInfo.Size,
				Created: imageInfo.Created,
			}

			if repo == "<none>:<none>" {
				repo = imageInfo.ID
				images[repo] = info
				break
			}

			images[repo] = info
		}
	}

	return images, nil
}
