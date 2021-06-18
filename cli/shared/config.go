package shared

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// initConfig configures viper from environment variables and configuration files.
func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetDefault("k8s.wait_timeout", "60s")

	viper.SetDefault("helm.install_timeout", "120s")

	viper.SetDefault("fabric.orderer_hostname_name", "orderer")

	viper.SetDefault("cli.success_emoji", "👍")
	viper.SetDefault("cli.ok_emoji", "👌")
	viper.SetDefault("cli.error_emoji", "\n❌")
	viper.SetDefault("cli.warning_emoji", "❗")
	viper.SetDefault("cli.info_emoji", "👉")

	viper.SetConfigType("yaml")
	viper.SetConfigName(".cli-config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./cli")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println(err)
	}
}
