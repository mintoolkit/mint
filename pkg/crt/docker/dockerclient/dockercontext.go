package dockerclient

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	DefaultContextName   = "default" //does not have a context file (pseudo-context)
	DockerConfigFileName = "config.json"
	DockerMetaFileName   = "meta.json"
	DockerEndpointType   = "docker"
)

// only relevant fields...
type DockerConfigFile struct {
	CurrentContext    string                 `json:"currentContext,omitempty"`
	CredentialsStore  string                 `json:"credsStore,omitempty"`
	CredentialHelpers map[string]string      `json:"credHelpers,omitempty"`
	AuthConfigs       map[string]AuthConfig  `json:"auths"`
	Proxies           map[string]ProxyConfig `json:"proxies,omitempty"`
}

type AuthConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"` //to auth user and get access token from registry
	RegistryToken string `json:"registrytoken,omitempty"` //bearer token for registry
	ServerAddress string `json:"serveraddress,omitempty"`
	Email         string `json:"email,omitempty"` //depricated field
}

type ProxyConfig struct {
	HTTPProxy  string `json:"httpProxy,omitempty"`
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	NoProxy    string `json:"noProxy,omitempty"`
	FTPProxy   string `json:"ftpProxy,omitempty"`
	AllProxy   string `json:"allProxy,omitempty"`
}

type DockerContextMetadata struct {
	Name     string
	Metadata struct {
		Description string
	}
	Endpoints map[string]EndpointMetadata
}

type EndpointMetadata struct {
	Host          string
	SkipTLSVerify bool
}

func ReadConfigFile(filePath string) (*DockerConfigFile, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	defer f.Close()
	var dockerConfig DockerConfigFile
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&dockerConfig)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	for addr, info := range dockerConfig.AuthConfigs {
		info.ServerAddress = addr

		if info.Auth != "" {
			info.Username, info.Password, err = decodeAuth(info.Auth)
			if err != nil {
				return nil, err
			}
		}

		dockerConfig.AuthConfigs[addr] = info
	}

	return &dockerConfig, nil
}

// copied from the CLI
func decodeAuth(input string) (string, string, error) {
	if input == "" {
		return "", "", nil
	}

	decLen := base64.StdEncoding.DecodedLen(len(input))
	decoded := make([]byte, decLen)
	authByte := []byte(input)
	n, err := base64.StdEncoding.Decode(decoded, authByte)
	if err != nil {
		return "", "", err
	}
	if n > decLen {
		return "", "", fmt.Errorf("Something went wrong decoding auth config")
	}
	userName, password, ok := strings.Cut(string(decoded), ":")
	if !ok || userName == "" {
		return "", "", fmt.Errorf("Invalid auth configuration file")
	}
	return userName, strings.Trim(password, "\x00"), nil
}

func ConfigFilePath() string {
	return filepath.Join(ConfigFileDir(), DockerConfigFileName)
}

func ConfigFileDir() string {
	output := os.Getenv(EnvDockerConfig)
	if output != "" {
		return output
	}

	return filepath.Join(UserHomeDir(), ".docker")
}

func ContextsDir() string {
	return filepath.Join(ConfigFileDir(), "contexts")
}

func ContextsMetaDir() string {
	return filepath.Join(ContextsDir(), "meta")
}

func ContextsTLSDir() string {
	return filepath.Join(ContextsDir(), "tls")
}

func ListContextsMetadata(dirPath string) ([]*DockerContextMetadata, error) {
	list, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var metaFilePaths []string
	for _, info := range list {
		if !info.IsDir() {
			continue
		}

		metaFilePaths = append(metaFilePaths,
			filepath.Join(dirPath, info.Name(), DockerMetaFileName))
	}

	var output []*DockerContextMetadata
	for _, fp := range metaFilePaths {
		cm, err := ReadContextMetadataFile(fp)
		if err != nil {
			return nil, err
		}

		if cm == nil {
			log.Debugf("dockerclient.ListContextsMetaGeneric: could not read '%s'", fp)
			continue
		}

		output = append(output, cm)
	}

	return output, nil
}

func ReadContextMetadataFile(filePath string) (*DockerContextMetadata, error) {
	log.Debugf("dockerclient.ReadContextMetadataFile(%s)", filePath)
	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	defer f.Close()
	var contextMetadata DockerContextMetadata
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&contextMetadata)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	log.Debugf("dockerclient.ReadContextMetadataFile(%s) - loaded context - %s",
		filePath, contextMetadata.Name)
	return &contextMetadata, nil
}

func (ref *DockerContextMetadata) Endpoint() *EndpointMetadata {
	output, found := ref.Endpoints[DockerEndpointType]
	if !found {
		return nil
	}

	return &output
}
