// Copyright (c) 2025 Broadcom. All Rights Reserved.
// Broadcom Confidential. The term "Broadcom" refers to Broadcom Inc.
// and/or its subsidiaries.

package ssh

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

// Auth represents ssh auth methods.
type Auth []ssh.AuthMethod

func configureAuth(password, privateKeyFile, passphrase string) (Auth, error) {
	if password != "" {
		return Password(password), nil
	} else if privateKeyFile != "" {
		return PrivateKey(privateKeyFile, passphrase)
	}
	return nil, fmt.Errorf("no private key/password found to configure SSH auth")
}

// Password returns password auth method.
func Password(pass string) Auth {
	return Auth{
		ssh.Password(pass),
	}
}

// PrivateKey returns auth method from private key with or without passphrase.
func PrivateKey(prvFile string, passphrase string) (Auth, error) {
	signer, err := getSigner(prvFile, passphrase)
	if err != nil {
		return nil, err
	}
	return Auth{
		ssh.PublicKeys(signer),
	}, nil
}

// getSigner returns ssh signer from private key file.
func getSigner(prvFile string, passphrase string) (ssh.Signer, error) {
	var (
		err    error
		signer ssh.Signer
	)
	privateKey, err := os.ReadFile(prvFile)
	if err != nil {
		return nil, fmt.Errorf("could not read private key: %w", err)
	}
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKey, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(privateKey)
	}
	return signer, err
}
