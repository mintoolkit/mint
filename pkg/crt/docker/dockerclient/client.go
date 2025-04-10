package dockerclient

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

const (
	EnvDockerConfig      = "DOCKER_CONFIG"
	EnvDockerContext     = "DOCKER_CONTEXT"
	EnvDockerAPIVer      = "DOCKER_API_VERSION"
	EnvDockerHost        = "DOCKER_HOST"
	EnvDockerTLSVerify   = "DOCKER_TLS_VERIFY"
	EnvDockerCertPath    = "DOCKER_CERT_PATH"
	UnixSocketPath       = "/var/run/docker.sock"
	UnixSocketAddr       = "unix:///var/run/docker.sock"
	unixUserSocketSuffix = ".docker/run/docker.sock"
)

var EnvVarNames = []string{
	EnvDockerHost,
	EnvDockerTLSVerify,
	EnvDockerCertPath,
	EnvDockerAPIVer,
	EnvDockerContext,
}

var (
	ErrNoDockerInfo = errors.New("no docker info")
)

func UserHomeDir() string {
	output, err := os.UserHomeDir()
	if err != nil {
		log.Debugf("dockerclient.UserHomeDir: os.UserHomeDir error - %v", err)
	}

	if output != "" {
		return output
	}

	info, err := user.Current()
	if err == nil {
		return info.HomeDir
	}

	log.Debugf("dockerclient.UserHomeDir: user.Current error - %v", err)
	return ""
}

func UserDockerSocket() string {
	home := UserHomeDir()
	return filepath.Join(home, unixUserSocketSuffix)
}

type SocketInfo struct {
	Address       string `json:"address"`
	FilePath      string `json:"file_path"`
	FileType      string `json:"type"`
	FilePerms     string `json:"perms"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
	TargetPerms   string `json:"target_perms,omitempty"`
	TargetType    string `json:"target_type,omitempty"`
	CanRead       bool   `json:"can_read"`
	CanWrite      bool   `json:"can_write"`
}

func getSocketInfo(filePath string) (*SocketInfo, error) {
	info := &SocketInfo{
		FileType: "file",
		FilePath: filePath,
	}

	fi, err := os.Lstat(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.os.Lstat(%s): error - %v", filePath, err)
		return nil, err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		info.SymlinkTarget, err = os.Readlink(info.FilePath)
		if err != nil {
			log.Errorf("dockerclient.getSocketInfo.os.Readlink(%s): error - %v", filePath, err)
			return nil, err
		}
		info.FileType = "symlink"
		info.FilePerms = fmt.Sprintf("%#o", fi.Mode().Perm())
		if info.SymlinkTarget != "" {
			tfi, err := os.Lstat(info.SymlinkTarget)
			if err != nil {
				log.Errorf("dockerclient.getSocketInfo.os.Lstat(%s): error - %v", info.SymlinkTarget, err)
				return nil, err
			}

			info.TargetPerms = fmt.Sprintf("%#o", tfi.Mode().Perm())
			if tfi.Mode()&os.ModeSymlink != 0 {
				info.TargetType = "symlink"
			}
		}
	}

	info.CanRead, err = fsutil.HasReadAccess(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.fsutil.HasReadAccess(%s): error - %v", info.FilePath, err)
		return nil, err
	}

	info.CanWrite, err = fsutil.HasWriteAccess(info.FilePath)
	if err != nil {
		log.Errorf("dockerclient.getSocketInfo.fsutil.HasWriteAccess(%s): error - %v", info.FilePath, err)
		return nil, err
	}

	return info, nil
}

func HasSocket(name string) bool {
	_, err := os.Stat(name)
	if err == nil || !os.IsNotExist(err) {
		return true
	}

	return false
}

func HasUserDockerSocket() bool {
	return HasSocket(UserDockerSocket())
}

func HasSystemDockerSocket() bool {
	return HasSocket(UnixSocketPath)
}

func GetUnixSocketAddr() (*SocketInfo, error) {
	//note: may move this to dockerutil

	//check the Desktop socket first
	userDockerSocket := UserDockerSocket()
	if _, err := os.Stat(userDockerSocket); err == nil {
		socketInfo, err := getSocketInfo(userDockerSocket)
		if err != nil {
			return nil, err
		}

		socketInfo.Address = fmt.Sprintf("unix://%s", userDockerSocket)
		log.Debugf("dockerclient.GetUnixSocketAddr(): found => %s", jsonutil.ToString(socketInfo))
		return socketInfo, nil
	}

	//then check the system socket next
	if _, err := os.Stat(UnixSocketPath); err == nil {
		socketInfo, err := getSocketInfo(UnixSocketPath)
		if err != nil {
			return nil, err
		}

		socketInfo.Address = UnixSocketAddr
		log.Debugf("dockerclient.GetUnixSocketAddr(): found => %s", jsonutil.ToString(socketInfo))
		return socketInfo, nil
	}

	return nil, fmt.Errorf("docker socket not found")
}

// New creates a new Docker client instance
func New(config *config.DockerClient) (*docker.Client, error) {
	var client *docker.Client
	var err error

	log.Tracef("dockerclient.New(config=%#v)", config)

	newTLSClient := func(host string, certPath string, verify bool, apiVersion string) (*docker.Client, error) {
		var ca []byte

		cert, err := os.ReadFile(filepath.Join(certPath, "cert.pem"))
		if err != nil {
			return nil, err
		}

		key, err := os.ReadFile(filepath.Join(certPath, "key.pem"))
		if err != nil {
			return nil, err
		}

		if verify {
			var err error
			ca, err = os.ReadFile(filepath.Join(certPath, "ca.pem"))
			if err != nil {
				return nil, err
			}
		}

		return docker.NewVersionedTLSClientFromBytes(host, cert, key, ca, apiVersion)
	}

	//NOTE:
	//go-dockerclient doesn't support DOCKER_CONTEXT natively
	//so we need to lookup the context first to extract its connection info
	var currentDockerContext string
	if dcf, err := ReadConfigFile(ConfigFilePath()); err == nil {
		if dcf == nil {
			log.Debug("dockerclient.New: No config file.")
		} else {
			currentDockerContext = dcf.CurrentContext
			log.Debugf("dockerclient.New: currentDockerContext - '%s'", currentDockerContext)
		}
	} else {
		log.Debugf("dockerclient.New: ReadConfigFile error - %v", err)
	}

	contextName := config.Context
	if contextName == "" {
		contextName = config.Env[EnvDockerContext]
	}
	if contextName == "" &&
		currentDockerContext != "" &&
		currentDockerContext != DefaultContextName {
		contextName = currentDockerContext
	}

	//note: don't use context host if the host parameter is specified explicitly
	var contextHost string
	var contextVerifyTLS bool
	if contextName != "" {
		log.Debugf("dockerclient.New: contextName - '%s' - loading contexts", contextName)
		cmList, err := ListContextsMetadata(ContextsMetaDir())
		if err == nil {
			var targetContext *DockerContextMetadata
			for _, cm := range cmList {
				if cm.Name == contextName {
					targetContext = cm
					break
				}
			}

			if targetContext != nil {
				info := targetContext.Endpoint()
				if info != nil {
					contextHost = info.Host
					if info.SkipTLSVerify {
						contextVerifyTLS = false
					} else {
						contextVerifyTLS = true
					}
				} else {
					log.Debugf("dockerclient.New: endpoint in target context ('%s') not found - %+v", contextName, targetContext)
				}
			} else {
				log.Debugf("dockerclient.New: target context ('%s') not found - %+v", contextName, cmList)
			}
		} else {
			log.Debugf("dockerclient.New: ListContextsMetadata error - %v", err)
		}
	}

	switch {
	case config.Host != "" &&
		config.UseTLS &&
		config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, true, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,verify) [1]")

	case config.Host != "" &&
		config.UseTLS &&
		!config.VerifyTLS &&
		config.TLSCertPath != "":
		client, err = newTLSClient(config.Host, config.TLSCertPath, false, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,no verify) [2]")

	case config.Host != "" &&
		!config.UseTLS:
		client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		if config.APIVersion != "" {
			client.SkipServerVersionCheck = true
		}

		log.Debug("dockerclient.New: new Docker client [3]")

	case config.Host == "" &&
		!config.VerifyTLS &&
		config.Env[EnvDockerTLSVerify] == "1" &&
		config.Env[EnvDockerCertPath] != "" &&
		config.Env[EnvDockerHost] != "":
		client, err = newTLSClient(config.Env[EnvDockerHost], config.Env[EnvDockerCertPath], false, config.APIVersion)
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (TLS,no verify) [4]")

	case config.Env[EnvDockerHost] != "":
		client, err = docker.NewClientFromEnv()
		if err != nil {
			return nil, err
		}

		log.Debug("dockerclient.New: new Docker client (env) [5]")

	case config.Host != "" && (strings.HasPrefix(config.Host, "/") || strings.HasPrefix(config.Host, "unix://")):
		log.Debugf("dockerclient.New: new Docker client - host(unix.socket)='%s'", config.Host)
		socketPath := strings.TrimPrefix(config.Host, "unix://")
		if !HasSocket(socketPath) {
			log.Errorf("dockerclient.New: new Docker client - no unix socket => %s", socketPath)
			return nil, ErrNoDockerInfo
		}

		socketInfo, err := getSocketInfo(socketPath)
		if err != nil {
			return nil, err
		}

		socketInfo.Address = fmt.Sprintf("unix://%s", socketPath)

		log.Debugf("dockerclient.New: new Docker client - unix socket => %s", jsonutil.ToString(socketInfo))
		if socketInfo.CanRead == false || socketInfo.CanWrite == false {
			return nil, fmt.Errorf("insufficient socket permissions (can_read=%v can_write=%v)", socketInfo.CanRead, socketInfo.CanWrite)
		}

		config.Host = socketInfo.Address
		client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		if config.APIVersion != "" {
			client.SkipServerVersionCheck = true
		}

	case config.Host == "" && config.Env[EnvDockerHost] == "" && contextHost != "":
		log.Debugf("dockerclient.New: new Docker client - from context ('%s') contextVerifyTLS=%v", contextHost, contextVerifyTLS)
		if strings.HasPrefix(contextHost, "/") ||
			strings.HasPrefix(contextHost, "unix://") {

			socketPath := strings.TrimPrefix(contextHost, "unix://")
			if !HasSocket(socketPath) {
				log.Errorf("dockerclient.New: new Docker client - from context ('%s') - no unix socket => '%s'", contextHost, socketPath)
				return nil, ErrNoDockerInfo
			}

			socketInfo, err := getSocketInfo(socketPath)
			if err != nil {
				return nil, err
			}

			socketInfo.Address = fmt.Sprintf("unix://%s", socketPath)

			if socketInfo.CanRead == false || socketInfo.CanWrite == false {
				return nil, fmt.Errorf("insufficient socket permissions (can_read=%v can_write=%v)", socketInfo.CanRead, socketInfo.CanWrite)
			}

			config.Host = socketInfo.Address
			client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
			if err != nil {
				return nil, err
			}

			if config.APIVersion != "" {
				client.SkipServerVersionCheck = true
			}

			log.Debugf("dockerclient.New: new Docker client - from context ('%s') - [7]", contextHost)
		} else {
			log.Debugf("dockerclient.New: new Docker client - from context - non-unix socket host (%s) [todo]", contextHost)
		}

	case config.Host == "" && config.Env[EnvDockerHost] == "":
		socketInfo, err := GetUnixSocketAddr()
		if err != nil {
			return nil, err
		}

		if socketInfo == nil || socketInfo.Address == "" {
			return nil, fmt.Errorf("no unix socket found")
		}

		if socketInfo.CanRead == false || socketInfo.CanWrite == false {
			return nil, fmt.Errorf("insufficient socket permissions (can_read=%v can_write=%v)", socketInfo.CanRead, socketInfo.CanWrite)
		}

		config.Host = socketInfo.Address
		client, err = docker.NewVersionedClient(config.Host, config.APIVersion)
		if err != nil {
			return nil, err
		}

		if config.APIVersion != "" {
			client.SkipServerVersionCheck = true
		}

		log.Debug("dockerclient.New: new Docker client (default) [6]")

	default:
		log.Debug("dockerclient.New: no docker info")
		return nil, ErrNoDockerInfo
	}

	if config.Env[EnvDockerHost] == "" {
		if err := os.Setenv(EnvDockerHost, config.Host); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("dockerclient.New: configured DOCKER_HOST env var")
	}

	if config.APIVersion != "" && config.Env[EnvDockerAPIVer] == "" {
		if err := os.Setenv(EnvDockerAPIVer, config.APIVersion); err != nil {
			errutil.WarnOn(err)
		}

		log.Debug("dockerclient.New: configured DOCKER_API_VERSION env var")
	}

	return client, nil
}
