package gen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/model"
	"github.com/timoth-y/fabnctl/pkg/util"
	"sigs.k8s.io/yaml"
)

// connectionCmd represents the connection command
var connectionCmd = &cobra.Command{
	Use:   "connection [artifacts path]",
	Short: "Generates connection configuration file",
	Long: `Generates connection configuration file

Examples:
  # Generate connection.yaml:
  fabnctl gen connection -f ./network-config.yaml -n edge-device -c supply-channel -o org1 ./artifacts

  # Generate connection.yaml with custom properties:
  fabnctl gen connection -f ./network-config.yaml -n edge-device -c supply-channel -o org1 /
    -x userID=user1,logging=debug ./artifacts
`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.Errorf(
				"%q requires exactly 1 argument: [artifacts path]", cmd.CommandPath(),
			)
		}
		return nil
	},
	RunE: shared.WithHandleErrors(func(cmd *cobra.Command, args []string) error {
		return genConnection(cmd, args[0])
	}),
}

func init() {
	cmd.AddCommand(connectionCmd)

	connectionCmd.Flags().StringP("org", "o", "", "Owner organization name (required)")
	connectionCmd.Flags().StringP("channel", "c", "", "Channel name (required)")
	connectionCmd.Flags().StringP("name", "n", "",
		"Connection profile name (default is {org}-connection)",
	)
	connectionCmd.Flags().String("description", "", "Connection profile description")
	connectionCmd.Flags().Float64P("version", "v", 1.0, "Version for connection profile")
	connectionCmd.Flags().StringToStringP("x-properties", "x", nil,
		"Custom extension properties that would be added to config as x-{key}: {values}",
	)

	_ = connectionCmd.MarkFlagRequired("org")
	_ = connectionCmd.MarkFlagRequired("channel")
}

func genConnection(cmd *cobra.Command, artifactsPath string) error {
	var (
		err         error
		configPath  string
		ownerOrg    string
		channel     string
		name        string
		desc        string
		version     float64
		xProperties map[string]string
		netConfig   model.NetworkConfig
	)

	// Parsing flags:
	if configPath, err = cmd.Flags().GetString("config"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse 'config' parameter")
	}

	if ownerOrg, err = cmd.Flags().GetString("org"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse required 'org' parameter")
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse required 'channel' parameter")
	}

	if name, err = cmd.Flags().GetString("name"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse 'name' parameter")
	}

	if desc, err = cmd.Flags().GetString("description"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse 'description' parameter")
	}

	if version, err = cmd.Flags().GetFloat64("version"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse 'version' parameter")
	}

	if xProperties, err = cmd.Flags().GetStringToString("x-properties"); err != nil {
		return errors.WithMessage(shared.ErrInvalidArgs, "failed to parse 'version' parameter")
	}

	if len(name) == 0 {
		name = fmt.Sprintf("%s-connection", ownerOrg)
	}

	if len(desc) == 0 {
		desc = fmt.Sprintf("Connection profile configuration for %s owned application", ownerOrg)
	}

	// Decoding network config file:
	configYaml, err := ioutil.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "missing configuration values on path %s", configPath)
	}

	if err = yaml.Unmarshal(configYaml, &netConfig); err != nil {
		return errors.Wrapf(err, "failed to decode config found on path: %s", configPath)
	}

	// Enrich network config with TLS CA certificates:
	var certPath = path.Join(
		artifactsPath,
		fmt.Sprintf(".crypto-config.%s", netConfig.Domain),
		"ordererOrganizations", netConfig.Domain,
		"tlsca", fmt.Sprintf("tlsca.%s-cert.pem", netConfig.Domain),
	)

	if cert, err := ioutil.ReadFile(certPath); err != nil {
		cmd.Printf(
			"%s TLS certificate for '%s' orderer not found on path '%s'\n",
			viper.GetString("cli.warning_emoji"), netConfig.Orderer.Name, certPath,
		)
	} else {
		netConfig.Orderer.TLSCert = string(cert)
	}

	// Filter channel and organization based on given channel ID:
	if ch := netConfig.GetChannel(channel); ch == nil {
		return errors.Errorf("Channel with ID '%s' isn't defined in given config '%s'", channel, configPath)
	} else {
		netConfig.Channels = []model.Channel{*ch}
	}

	var channelOrgs []model.Organization

	for i, org := range netConfig.Organizations {
		if netConfig.GetChannel(channel).HasOrganization(org.Name) {
			channelOrgs = append(channelOrgs, netConfig.Organizations[i])
		}
	}
	netConfig.Organizations = channelOrgs

	if org := netConfig.GetOrganization(ownerOrg); org == nil {
		return errors.Errorf("Organization with ID '%s' isn't a part of '%s' channel consortium", org, channel)
	}

	for i, org := range netConfig.Organizations {
		certPath = path.Join(
			artifactsPath,
			fmt.Sprintf(".crypto-config.%s", netConfig.Domain),
			"peerOrganizations", fmt.Sprintf("%s.%s", org.Hostname, netConfig.Domain),
			"tlsca", fmt.Sprintf("tlsca.%s.%s-cert.pem", org.Hostname, netConfig.Domain),
		)

		if cert, err := ioutil.ReadFile(certPath); err != nil {
			cmd.Printf(
				"%s TLS certificate for '%s' organization not found on path '%s'\n",
				viper.GetString("cli.warning_emoji"), org.Name, certPath,
			)
		} else {
			netConfig.Organizations[i].TLSCert = string(cert)
		}

		certPath = path.Join(
			artifactsPath,
			fmt.Sprintf(".crypto-config.%s", netConfig.Domain),
			"peerOrganizations", fmt.Sprintf("%s.%s", org.Hostname, netConfig.Domain),
			"ca", fmt.Sprintf("ca.%s.%s-cert.pem", org.Hostname, netConfig.Domain),
		)

		if cert, err := ioutil.ReadFile(certPath); err != nil {
			cmd.Printf(
				"%s TLS certificate for '%s' organization's CA not found on path '%s'\n",
				viper.GetString("cli.warning_emoji"), org.Name, certPath,
			)
		} else {
			netConfig.Organizations[i].CertAuthority.TLSCert = string(cert)
		}
	}

	// Values for template rendering:
	type ConnectionValues struct {
		Name string
		Description string
		Version string
		OwnerOrg string
		Channel string
		model.NetworkConfig
		XProperties map[string]string
	}

	values := ConnectionValues{
		Name:          name,
		Description:   desc,
		Version:       util.Vtoa(version),
		OwnerOrg:      ownerOrg,
		Channel:       channel,
		NetworkConfig: netConfig,
		XProperties:   xProperties,
	}

	var (
		tpl = template.Must(
			template.New("connection").
				Funcs(sprig.TxtFuncMap()).
				ParseFiles(
					path.Join(viper.GetString("installation_path"), "template/connection.goyaml"),
				),
		)
	)

	// Render template in connection.yaml file
	file, err := os.Create("connection.yaml")
	if err != nil {
		return errors.Wrap(err, "failed to create file 'connection.yaml'")
	}
	defer func() {
		_ = file.Close()
	}()

	if err = tpl.ExecuteTemplate(file, "connection.goyaml", values); err != nil {
		return errors.Wrap(err, "failed to render connection config")
	}

	cmd.Println("🎉 Connection config generation done!")

	return nil
}

