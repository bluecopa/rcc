package conda

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/robocorp/rcc/blobs"
	"github.com/robocorp/rcc/common"
	"github.com/robocorp/rcc/pathlib"
	"github.com/robocorp/rcc/pretty"
	"github.com/robocorp/rcc/settings"
)

const (
	Newline        = "\n"
	binSuffix      = "/bin"
	activateScript = `#!/bin/bash

export MAMBA_ROOT_PREFIX={{.MambaRootPrefix}}
eval "$('{{.Micromamba}}' shell activate -s bash -p {{.Live}})"
"{{.Rcc}}" internal env -l after
`
	commandSuffix = ".sh"
)

var (
	Shell          = []string{"bash", "--noprofile", "--norc", "-i"}
	FileExtensions = []string{"", ".sh"}
)

func CondaEnvironment() []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("MAMBA_ROOT_PREFIX=%s", common.MambaRootPrefix()))
	tempFolder := common.RobocorpTemp()
	env = append(env, fmt.Sprintf("TEMP=%s", tempFolder))
	env = append(env, fmt.Sprintf("TMP=%s", tempFolder))
	return injectNetworkEnvironment(env)
}

func BinMicromamba() string {
	location := common.ExpandPath(filepath.Join(common.MicromambaLocation(), blobs.MicromambaVersion()))
	err := pathlib.EnsureDirectoryExists(location)
	if err != nil {
		pretty.Warning("Problem creating %q, reason: %v", location, err)
	}
	return common.ExpandPath(filepath.Join(location, "micromamba"))
}

func CondaPaths(prefix string) []string {
	return []string{prefix + binSuffix}
}

func MicromambaLink() string {
	return settings.Global.DownloadsLink(micromambaLink("macos64", "micromamba"))
}

func IsWindows() bool {
	return false
}

func HasLongPathSupport() bool {
	return true
}

func EnforceLongpathSupport() error {
	return nil
}
