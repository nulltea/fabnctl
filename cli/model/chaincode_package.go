package model

type ChaincodeMetadata struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type ChaincodeConnection struct {
	Address string `json:"address"`
	DialTimeout string `json:"dial_timeout"`
	TLSRequired bool `json:"tls_required"`
	ClientAuthRequired bool `json:"client_auth_required"`
	ClientKey string `json:"client_key"`
	ClientCert string `json:"client_cert"`
	RootCert string `json:"root_cert"`
}
