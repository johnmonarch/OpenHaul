package credentials

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/zalando/go-keyring"
)

const service = "openhaulguard"

const (
	UserFMCSAWebKey     = "fmcsa_webkey"
	UserSocrataAppToken = "socrata_app_token"
)

type Store struct {
	Home string
}

type secretFile struct {
	FMCSAWebKey     string `toml:"fmcsa_web_key"`
	SocrataAppToken string `toml:"socrata_app_token"`
}

func (s Store) Get(user string) (string, error) {
	switch user {
	case UserFMCSAWebKey:
		if v := os.Getenv("OHG_FMCSA_WEBKEY"); v != "" {
			return v, nil
		}
		if v := os.Getenv("OHG_FMCSA_WEB_KEY"); v != "" {
			return v, nil
		}
	case UserSocrataAppToken:
		if v := os.Getenv("OHG_SOCRATA_APP_TOKEN"); v != "" {
			return v, nil
		}
	}
	v, err := keyring.Get(service, user)
	if err == nil && v != "" {
		return v, nil
	}
	return s.getFallback(user)
}

func (s Store) Set(user, value string) error {
	if err := keyring.Set(service, user, value); err == nil {
		return nil
	}
	return s.setFallback(user, value)
}

func (s Store) Delete(user string) error {
	_ = keyring.Delete(service, user)
	path := filepath.Join(s.Home, "secrets.toml")
	var sf secretFile
	_, _ = toml.DecodeFile(path, &sf)
	switch user {
	case UserFMCSAWebKey:
		sf.FMCSAWebKey = ""
	case UserSocrataAppToken:
		sf.SocrataAppToken = ""
	}
	return s.writeFallback(sf)
}

func (s Store) getFallback(user string) (string, error) {
	path := filepath.Join(s.Home, "secrets.toml")
	var sf secretFile
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	if _, err := toml.DecodeFile(path, &sf); err != nil {
		return "", err
	}
	switch user {
	case UserFMCSAWebKey:
		if sf.FMCSAWebKey != "" {
			return sf.FMCSAWebKey, nil
		}
	case UserSocrataAppToken:
		if sf.SocrataAppToken != "" {
			return sf.SocrataAppToken, nil
		}
	}
	return "", errors.New("credential not found")
}

func (s Store) setFallback(user, value string) error {
	path := filepath.Join(s.Home, "secrets.toml")
	var sf secretFile
	_, _ = toml.DecodeFile(path, &sf)
	switch user {
	case UserFMCSAWebKey:
		sf.FMCSAWebKey = value
	case UserSocrataAppToken:
		sf.SocrataAppToken = value
	}
	return s.writeFallback(sf)
}

func (s Store) writeFallback(sf secretFile) error {
	if err := os.MkdirAll(s.Home, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.Home, "secrets.toml")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(sf)
}
