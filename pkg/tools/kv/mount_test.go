// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package kv

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/vault/api"
)

func TestLookupMount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/internal/ui/mounts/kv2":
			_, _ = w.Write([]byte(`{"data":{"type":"kv","path":"kv2/","options":{"version":"2"}}}`))
		case "/v1/sys/internal/ui/mounts/kv1":
			_, _ = w.Write([]byte(`{"data":{"type":"kv","path":"kv1/","options":null}}`))
		case "/v1/sys/internal/ui/mounts/denied":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
		case "/v1/sys/mounts":
			t.Error("sys/mounts must not be called")
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[]}`))
		}
	}))
	defer srv.Close()

	vault, err := api.NewClient(&api.Config{Address: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	vault.SetToken("test")

	m, err := lookupMount(vault, "kv2/")
	if err != nil || m == nil || m.Type != "kv" || m.Options["version"] != "2" {
		t.Fatalf("kv2: got %+v, %v", m, err)
	}
	m, err = lookupMount(vault, "kv1")
	if err != nil || m == nil || m.Options["version"] != "" {
		t.Fatalf("kv1: got %+v, %v", m, err)
	}
	m, err = lookupMount(vault, "missing")
	if err != nil || m != nil {
		t.Fatalf("missing: got %+v, %v", m, err)
	}
	if _, err = lookupMount(vault, "denied"); err == nil {
		t.Fatal("denied: want an error")
	}
}
