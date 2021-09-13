package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/timoth-y/fabnctl/pkg/term"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	sshTerminal "golang.org/x/crypto/ssh/terminal"
)

func sshAgentAuthMethod() (ssh.AuthMethod, error) {
	ag, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(agent.NewClient(ag).Signers), nil
}

func publicKeyAuthMethod(path string) (ssh.AuthMethod, context.CancelFunc, error) {
	noopCloseFunc := func() { }

	key, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, noopCloseFunc, fmt.Errorf("unable to read file: %s, %s", path, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		if _, ok := err.(*ssh.PassphraseMissingError); !ok {
			return nil, noopCloseFunc, fmt.Errorf("unable to parse private key: %s", err.Error())
		}

		a, cl := sshAgent(path)
		if a != nil {
			return a, cl, nil
		}

		defer cl()

		fmt.Printf("Enter passphrase for '%s': ", path)
		STDIN := int(os.Stdin.Fd())
		bytePassword, _ := sshTerminal.ReadPassword(STDIN)

		fmt.Println()

		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, bytePassword)
		if err != nil {
			return nil, noopCloseFunc, fmt.Errorf("parse private key with passphrase failed: %s", err)
		}
	}

	return ssh.PublicKeys(signer), noopCloseFunc, nil
}

func sshAgent(publicKeyPath string) (ssh.AuthMethod, context.CancelFunc) {
	var (
		wrapWithErrLogFunc = func(f func() error) context.CancelFunc {
			return func() {
				if err := f(); err != nil {
					term.NewLogger().Error(err, "error during closing SSH agent")
				}
			}
		}
	)
	if sshAgentConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		sshAgent := agent.NewClient(sshAgentConn)

		keys, _ := sshAgent.List()
		if len(keys) == 0 {
			return nil, wrapWithErrLogFunc(sshAgentConn.Close)
		}

		pubKey, err := ioutil.ReadFile(publicKeyPath)
		if err != nil {
			return nil, wrapWithErrLogFunc(sshAgentConn.Close)
		}

		authKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKey)
		if err != nil {
			return nil, wrapWithErrLogFunc(sshAgentConn.Close)
		}
		parsedKey := authKey.Marshal()

		for _, key := range keys {
			if bytes.Equal(key.Blob, parsedKey) {
				return ssh.PublicKeysCallback(sshAgent.Signers), wrapWithErrLogFunc(sshAgentConn.Close)
			}
		}
	}
	return nil, func() { }
}
