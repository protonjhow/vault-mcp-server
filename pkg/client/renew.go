// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
)

// startRenewal keeps the client's token renewed in the background, if it is
// renewable, until stop is called or OpenBao stops extending it.
func startRenewal(client *api.Client, logger *log.Logger) (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	go renewToken(client, logger, done)
	return func() { once.Do(func() { close(done) }) }
}

func renewToken(client *api.Client, logger *log.Logger, done <-chan struct{}) {
	secret, err := client.Auth().Token().LookupSelf()
	if err != nil {
		logger.WithError(err).Info("Vault token renewal off: token lookup failed")
		return
	}
	if renewable, _ := secret.TokenIsRenewable(); !renewable {
		logger.Info("Vault token renewal off: token is not renewable")
		return
	}
	ttl, _ := secret.TokenTTL()
	creationTTL, _ := secret.Data["creation_ttl"].(json.Number)
	increment, _ := creationTTL.Int64()

	watcher, err := client.NewLifetimeWatcher(&api.LifetimeWatcherInput{
		Secret: &api.Secret{Auth: &api.SecretAuth{
			ClientToken:   client.Token(),
			Renewable:     true,
			LeaseDuration: int(ttl.Seconds()),
		}},
		Increment: int(increment),
	})
	if err != nil {
		logger.WithError(err).Info("Vault token renewal off: cannot start watcher")
		return
	}
	go watcher.Start()
	defer watcher.Stop()

	expires := time.Now().Add(ttl)
	logger.WithField("ttl", ttl).Info("Vault token renewal on")
	for {
		select {
		case <-done:
			return
		case r := <-watcher.RenewCh():
			lease := time.Duration(r.Secret.Auth.LeaseDuration) * time.Second
			expires = r.RenewedAt.Add(lease)
			logger.WithField("ttl", lease).Debug("Renewed Vault token")
		case err := <-watcher.DoneCh():
			entry := logger.WithField("expires_in", time.Until(expires).Round(time.Second))
			if err != nil {
				entry.WithError(err).Warn("Vault token renewal stopped: renewal failed, the token will expire")
			} else {
				entry.Warn("Vault token renewal stopped: the token has reached its maximum lifetime and will expire")
			}
			return
		}
	}
}
