package model

type NetworkConfig struct {
	Domain string `yaml:"domain" json:"domain"`
	Orderer Orderer `yaml:"orderer" json:"orderer"`
	Organizations []Organization `yaml:"organizations" json:"organizations"`
	Channels []Channel `yaml:"channels" json:"channels"`

	orgMap      map[string]*Organization
	channelsMap map[string]*Channel
}

type Orderer struct {
	Name     string `yaml:"name" json:"name"`
	Hostname string `yaml:"hostname" json:"hostname"`
	TLSCert  string `yaml:"-" json:"-"`
}

type Organization struct {
	Name     string `yaml:"name" json:"name"`
	Hostname string `yaml:"hostname" json:"hostname"`
	MspID    string `yaml:"mspID" json:"mspID"`
	Peers []struct{
		Hostname string `yaml:"hostname" json:"hostname"`
	}
	TLSCert  string `yaml:"-" json:"-"`
	CertAuthority struct{
		TLSCert string `yaml:"-" json:"-"`
	} `yaml:"cert_authority" json:"cert_authority"`
}

type Channel struct {
	ChannelID string `yaml:"channelID" json:"channelID"`
	Organizations []string `yaml:"organizations" json:"organizations"`
}

func (n NetworkConfig) GetChannel(channelID string) *Channel {
	if n.channelsMap == nil {
		n.channelsMap = make(map[string]*Channel)
		for i, ch := range n.Channels {
			n.channelsMap[ch.ChannelID] = &n.Channels[i]
		}
	}

	if ch, ok := n.channelsMap[channelID]; ok {
		return ch
	}

	return nil
}

func (c *Channel) HasOrganization(name string) bool {
	for _, org := range c.Organizations {
		if org == name {
			return true
		}
	}

	return false
}

func (n NetworkConfig) GetOrganization(orgID string) *Organization {
	if n.orgMap == nil {
		n.orgMap = make(map[string]*Organization)
		for i, org := range n.Organizations {
			n.orgMap[org.MspID] = &n.Organizations[i]
		}
	}

	if org, ok := n.orgMap[orgID]; ok {
		return org
	}

	return nil
}
