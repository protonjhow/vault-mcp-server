// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/sirupsen/logrus/hooks/test"
)

// fakeBao serves auth/token/lookup-self with the given status and renewable
// flag (ttl and creation_ttl of 3s) and counts auth/token/renew-self calls.
func fakeBao(t *testing.T, lookupStatus int, renewable bool) (*api.Client, *atomic.Int32) {
	t.Helper()
	var renewals atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/token/lookup-self":
			w.WriteHeader(lookupStatus)
			if lookupStatus != http.StatusOK {
				fmt.Fprint(w, `{"errors":["permission denied"]}`)
				return
			}
			fmt.Fprintf(w, `{"data":{"renewable":%t,"ttl":3,"creation_ttl":3}}`, renewable)
		case "/v1/auth/token/renew-self":
			renewals.Add(1)
			fmt.Fprint(w, `{"auth":{"client_token":"test-token","renewable":true,"lease_duration":3}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	config := api.DefaultConfig()
	config.Address = srv.URL
	client, err := api.NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken("test-token")
	return client, &renewals
}

func TestRenewalRenewsRenewableToken(t *testing.T) {
	client, renewals := fakeBao(t, http.StatusOK, true)
	logger, _ := test.NewNullLogger()

	stop := startRenewal(client, logger)
	defer stop()

	time.Sleep(5 * time.Second)
	if n := renewals.Load(); n < 2 {
		t.Fatalf("got %d renewals in 5s, want at least 2", n)
	}
}

func TestRenewalSkipsNonRenewableToken(t *testing.T) {
	client, renewals := fakeBao(t, http.StatusOK, false)
	logger, _ := test.NewNullLogger()

	stop := startRenewal(client, logger)
	defer stop()

	time.Sleep(time.Second)
	if n := renewals.Load(); n != 0 {
		t.Fatalf("got %d renewals for a non-renewable token, want 0", n)
	}
}

func TestRenewalSkipsWhenLookupDenied(t *testing.T) {
	client, renewals := fakeBao(t, http.StatusForbidden, true)
	logger, _ := test.NewNullLogger()

	stop := startRenewal(client, logger)
	defer stop()

	time.Sleep(time.Second)
	if n := renewals.Load(); n != 0 {
		t.Fatalf("got %d renewals after a denied lookup, want 0", n)
	}
}

func TestRenewalStopEndsRenewalQuietly(t *testing.T) {
	client, renewals := fakeBao(t, http.StatusOK, true)
	logger, hook := test.NewNullLogger()

	stop := startRenewal(client, logger)
	deadline := time.Now().Add(2 * time.Second)
	for renewals.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no renewal within 2s")
		}
		time.Sleep(50 * time.Millisecond)
	}

	stop()
	before := renewals.Load()
	time.Sleep(3 * time.Second)
	if n := renewals.Load(); n != before {
		t.Fatalf("got %d renewals after stop, want 0", n-before)
	}
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, "renewal stopped") {
			t.Fatalf("logged %q after a deliberate stop", e.Message)
		}
	}
}
