// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kv

import (
	"strings"

	"github.com/hashicorp/vault/api"
)

// lookupMount reads one mount's type and options from
// sys/internal/ui/mounts/<mount>, which answers for any token with a
// capability on that mount. sys/mounts needs read on sys/mounts itself, which
// ordinary policies rarely grant (issue #98). The Vault and OpenBao CLIs use
// the same endpoint before every kv command. Returns nil when the mount does
// not exist.
func lookupMount(vault *api.Client, mount string) (*api.MountOutput, error) {
	secret, err := vault.Logical().Read("sys/internal/ui/mounts/" + strings.Trim(mount, "/"))
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}
	m := &api.MountOutput{Options: map[string]string{}}
	m.Type, _ = secret.Data["type"].(string)
	if opts, ok := secret.Data["options"].(map[string]interface{}); ok {
		for k, v := range opts {
			if s, ok := v.(string); ok {
				m.Options[k] = s
			}
		}
	}
	return m, nil
}
