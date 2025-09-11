package staticservernode

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/erh/vmodutils"
	"github.com/hashicorp/go-extract"
	generic "go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var (
	Server           = resource.NewModel("viam", "static-server-node", "server")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(generic.API, Server,
		resource.Registration[resource.Resource, *Config]{
			Constructor: newStaticServerNodeServer,
		},
	)
}

type Config struct {
	Path         string  `json:"path"`
	AccessToken  *string `json:"access_token,omitempty"`
	NodeVersion  *string `json:"node_version,omitempty"`
	BuildCommand *string `json:"build_command,omitempty"`
	BuildDir     *string `json:"build_directory,omitempty"`
	Port         *int    `json:"port,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if len(strings.TrimSpace(cfg.Path)) == 0 {
		return nil, nil, errors.New("path is required")
	}
	if !strings.HasPrefix(cfg.Path, "git+") {
		return nil, nil, errors.New("only git paths are currently supported")
	}
	return nil, nil, nil
}

type staticServerNodeServer struct {
	resource.AlwaysRebuild

	name resource.Name

	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()
}

func newStaticServerNodeServer(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (resource.Resource, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	return NewServer(ctx, deps, rawConf.ResourceName(), conf, logger)

}

func NewServer(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (resource.Resource, error) {

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &staticServerNodeServer{
		name:       name,
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	nodeDir, err := s.downloadNode(s.cancelCtx)
	if err != nil {
		return nil, err
	}

	projectDir, err := s.getProjectDir(s.cancelCtx)
	if err != nil {
		return nil, err
	}

	buildFS, err := s.buildAndGetFS(s.cancelCtx, nodeDir, projectDir)
	if err != nil {
		return nil, err
	}

	port := 8888
	if conf.Port != nil {
		port = *conf.Port
	}
	return vmodutils.NewWebModuleAndStart(name, buildFS, logger, port)
}

func (s *staticServerNodeServer) Name() resource.Name {
	return s.name
}

func (s *staticServerNodeServer) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *staticServerNodeServer) Close(context.Context) error {
	// Put close code here
	s.cancelFunc()
	return nil
}

func (s *staticServerNodeServer) getCacheDir() (string, error) {
	cacheDir := filepath.Join(os.TempDir(), "viam", string(Server.Family.Namespace), Server.Family.Name, Server.Name, s.name.Name)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}
	return cacheDir, nil
}

func (s *staticServerNodeServer) getNodeURL() string {
	nodeVersion := s.getNodeVersion()
	fileName, ext := s.getNodeFilenameAndExt()
	return fmt.Sprintf("https://nodejs.org/dist/%s/%s.%s", nodeVersion, fileName, ext)
}

func (s *staticServerNodeServer) getNodeVersion() string {
	nodeVersion := "22.19.0"
	if !isStringRefEmpty(s.cfg.NodeVersion) {
		nodeVersion = *s.cfg.NodeVersion
	}
	if !strings.HasPrefix(nodeVersion, "v") {
		nodeVersion = "v" + nodeVersion
	}
	return nodeVersion
}

func (s *staticServerNodeServer) getNodeFilenameAndExt() (string, string) {
	platform := runtime.GOOS
	arch := runtime.GOARCH

	if strings.ToLower(arch) == "amd64" {
		arch = "x64"
	}
	if strings.ToLower(arch) == "386" {
		arch = "x86"
	}

	suffix := "tar.xz"
	if strings.ToLower(platform) == "darwin" {
		suffix = "tar.gz"
	}
	if strings.ToLower(platform) == "windows" {
		suffix = "zip"
	}

	return fmt.Sprintf("node-%s-%s-%s", s.getNodeVersion(), platform, arch), suffix
}

func (s *staticServerNodeServer) isNodeDownloaded() (bool, string) {
	cache, err := s.getCacheDir()
	if err != nil {
		return false, ""
	}

	nodeFileName, _ := s.getNodeFilenameAndExt()
	filePath := filepath.Join(cache, nodeFileName)
	if _, err = os.Stat(filePath); err == nil {
		return true, filePath
	}

	return false, ""
}

func (s *staticServerNodeServer) downloadNode(ctx context.Context) (string, error) {
	s.logger.Debug("Downloading Node...")
	if downloaded, nodePath := s.isNodeDownloaded(); downloaded {
		s.logger.Debugf("\tNode already downloaded at %s", nodePath)
		return nodePath, nil
	}

	cacheDir, err := s.getCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	} else {
		if contents, err := os.ReadDir(cacheDir); err == nil {
			for _, c := range contents {
				os.RemoveAll(filepath.Join(cacheDir, c.Name()))
			}
		}
	}

	client := grab.NewClient()
	req, err := grab.NewRequest(cacheDir, s.getNodeURL())
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	resp := client.Do(req)

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			s.logger.Debugf("\tDownloaded %v/%v bytes (%.2f%%)\n", resp.BytesComplete(), resp.Size(), 100*resp.Progress())
		case <-resp.Done:
			break Loop
		}
	}

	if err := resp.Err(); err != nil {
		return "", err
	}

	f, err := os.Open(resp.Filename)
	if err != nil {
		return "", err
	}

	if err := extract.Unpack(ctx, cacheDir, f, extract.NewConfig(extract.WithCreateDestination(true), extract.WithOverwrite(true))); err != nil {
		return "", err
	}

	fileName, _ := s.getNodeFilenameAndExt()
	execPath := filepath.Join(cacheDir, fileName)
	return execPath, nil
}

func (s *staticServerNodeServer) getProjectDir(ctx context.Context) (string, error) {
	if strings.HasPrefix(s.cfg.Path, "git+") {
		gitDir, err := s.downloadGitRepo(ctx)
		if err != nil {
			return "", err
		}
		return gitDir, nil
	}
	return "", errors.New("UNREACHABLE CODE PATH")
}

type gitURLInfo struct {
	Owner      string
	Repository string
	Ref        string
}

func parseGitURL(url string) (*gitURLInfo, error) {
	re := regexp.MustCompile(`^git\+https:\/\/github\.com\/(?P<owner>[\w-]+)\/(?P<repo>[\w.-]+)(?:#(?P<ref>[\w\/-]+))?$`)

	if !re.MatchString(url) {
		return nil, fmt.Errorf("URL does not match the expected format")
	}

	matches := re.FindStringSubmatch(url)
	subexpNames := re.SubexpNames()

	info := &gitURLInfo{}
	for i, name := range subexpNames {
		if i > 0 && i < len(matches) {
			switch name {
			case "owner":
				info.Owner = matches[i]
			case "repo":
				info.Repository = matches[i]
			case "ref":
				info.Ref = matches[i]
			}
		}
	}

	if info.Ref == "" {
		info.Ref = "main"
	}

	return info, nil
}

func (s *staticServerNodeServer) downloadGitRepo(ctx context.Context) (string, error) {
	gitInfo, err := parseGitURL(s.cfg.Path)
	if err != nil {
		return "", err
	}
	gitUrl := fmt.Sprintf("https://api.github.com/repos/%s/%s/zipball/%s", gitInfo.Owner, gitInfo.Repository, gitInfo.Ref)
	cacheDir, err := s.getCacheDir()
	if err != nil {
		return "", err
	}

	client := grab.NewClient()
	req, err := grab.NewRequest(cacheDir, gitUrl)
	if !isStringRefEmpty(s.cfg.AccessToken) {
		req.HTTPRequest.Header.Add("Authorization", "token "+strings.TrimSpace(*s.cfg.AccessToken))
	}
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	resp := client.Do(req)
	s.logger.Debugf("Downloading git repository to %s", cacheDir)

	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			s.logger.Debugf("\tDownloaded %v/%v bytes (%.2f%%)", resp.BytesComplete(), resp.Size(), 100*resp.Progress())
		case <-resp.Done:
			break Loop
		}
	}

	if err := resp.Err(); err != nil {
		return "", err
	}

	f, err := os.Open(resp.Filename)
	if err != nil {
		return "", err
	}

	if err := extract.Unpack(ctx, cacheDir, f, extract.NewConfig(extract.WithCreateDestination(true), extract.WithOverwrite(true))); err != nil {
		return "", err
	}

	return strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())), nil
}

func (s *staticServerNodeServer) buildAndGetFS(ctx context.Context, nodeDir string, projectDir string) (fs.FS, error) {
	npm := filepath.Join(nodeDir, "bin", "npm")
	if runtime.GOOS == "windows" {
		npm = filepath.Join(nodeDir, "npm")
	}
	installCmd := exec.Command(npm, "install")
	installCmd.Dir = projectDir
	s.logger.Debug("Installing npm packages...")
	err := installCmd.Run()
	if err != nil {
		return nil, err
	}

	npx := filepath.Join(nodeDir, "bin", "npx")
	if runtime.GOOS == "windows" {
		npm = filepath.Join(nodeDir, "npx")
	}
	buildCommand := "build"
	if !isStringRefEmpty(s.cfg.BuildCommand) {
		buildCommand = *s.cfg.BuildCommand
	}

	envVars := generateEnvVars(projectDir)
	buildCommand = fmt.Sprintf("--yes cross-env %s %s run %s", envVars, npm, buildCommand)
	args := strings.Split(buildCommand, " ")
	buildCmd := exec.Command(npx, args...)
	buildCmd.Dir = projectDir
	s.logger.Debugf("Building with \"npm %s\"...", buildCommand)
	err = buildCmd.Run()
	if err != nil {
		return nil, err
	}

	buildDir := "dist"
	if !isStringRefEmpty(s.cfg.BuildDir) {
		buildDir = *s.cfg.BuildDir
	}
	buildDir = filepath.Join(projectDir, buildDir)
	_, err = os.Stat(buildDir)
	if err != nil {
		return nil, err
	}

	return os.DirFS(buildDir), nil
}

func generateEnvVars(projectDir string) string {
	envVarPrefix := ""
	builder := getBuilder(projectDir)
	if builder == "vite" {
		envVarPrefix = "VITE_"
	}
	return fmt.Sprintf("%sVIAM_API_KEY=%s %sVIAM_API_KEY_ID=%s", envVarPrefix, os.Getenv("VIAM_API_KEY"), envVarPrefix, os.Getenv("VIAM_API_KEY_ID"))
}

func getBuilder(projectDir string) string {
	packageJsonBytes, err := os.ReadFile(projectDir + "package.json")
	if err != nil {
		return "vite"
	}
	packageJson := string(packageJsonBytes)
	if strings.Contains(packageJson, "rollup") {
		return "rollup"
	}
	if strings.Contains(packageJson, "wepback") {
		return "webpack"
	}
	return "vite"
}

func isStringRefEmpty(str *string) bool {
	return str == nil || len(strings.TrimSpace(*str)) == 0
}
