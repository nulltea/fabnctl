package model

// NetworkConfig defines network deployment configuration structure.
type NetworkConfig struct {
	Domain string `yaml:"domain" json:"domain"`
	Orderer Orderer `yaml:"orderer" json:"orderer"`
	Organizations []Organization `yaml:"organizations" json:"organizations"`
	Channels []Channel `yaml:"channels" json:"channels"`

	orgMap      map[string]*Organization
	channelsMap map[string]*Channel
}

// Orderer defines orderer block structure from NetworkConfig.
type Orderer struct {
	Name     string `yaml:"name" json:"name"`
	Hostname string `yaml:"hostname" json:"hostname"`
	TLSCert  string `yaml:"-" json:"-"`
}

// Organization defines organization block structure from NetworkConfig.
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

// Channel defines channel block structure from NetworkConfig.
type Channel struct {
	ChannelID string `yaml:"channelID" json:"channelID"`
	Organizations []string `yaml:"organizations" json:"organizations"`
}

// GetChannel finds single Channel config in the NetworkConfig.
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

// HasOrganization determines whether the Organization is a part of Channel in the NetworkConfig.
func (c *Channel) HasOrganization(name string) bool {
	for _, org := range c.Organizations {
		if org == name {
			return true
		}
	}

	return false
}

// GetOrganization finds single Organization config in the NetworkConfig.
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
