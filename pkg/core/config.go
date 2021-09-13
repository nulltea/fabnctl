package core

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"k8s.io/client-go/util/homedir"
)

// init configures viper from environment variables and configuration files.
func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetDefault("k8s.wait_timeout", "60s")

	viper.SetDefault("helm.install_timeout", "120s")
	viper.SetDefault("helm.charts_path", path.Join(homedir.HomeDir(), "fabnctl", "charts"))

	viper.SetDefault("fabric.orderer_hostname_name", "orderer")

	viper.Set("cli.success_emoji", "👍")
	viper.Set("cli.ok_emoji", "👌")
	viper.Set("cli.error_emoji", "\n❌")
	viper.Set("cli.warning_emoji", "❗")
	viper.Set("cli.info_emoji", "👉")

	viper.SetDefault("installation_path", path.Join(homedir.HomeDir(), "fabnctl"))

	viper.SetConfigType("yaml")
	viper.SetConfigName(".cli-config")
	viper.AddConfigPath(".")
	viper.AddConfigPath(viper.GetString("installation_path"))

	if installPath, err := GetInstallationPath(); err == nil {
		viper.AddConfigPath(installPath)
		viper.AddConfigPath(path.Join(installPath, "cli"))
	}

	_ = viper.ReadInConfig()
}

// GetInstallationPath determines cli binary installation path
func GetInstallationPath() (string, error) {
	return filepath.Abs(filepath.Dir(os.Args[0]))
}
