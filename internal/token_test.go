// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRetrieveToken_InParams(t *testing.T) {
	styleCache := new(AuthStyleCache)
	const clientID = "client-id"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.FormValue("client_id"), clientID; got != want {
			t.Errorf("client_id = %q; want %q", got, want)
		}
		if got, want := r.FormValue("client_secret"), ""; got != want {
			t.Errorf("client_secret = %q; want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token": "ACCESS_TOKEN", "token_type": "bearer"}`)
	}))
	defer ts.Close()
	_, err := RetrieveToken(context.Background(), clientID, "", ts.URL, url.Values{}, AuthStyleInParams, styleCache)
	if err != nil {
		t.Errorf("RetrieveToken = %v; want no error", err)
	}
}

func TestRetrieveTokenWithContexts(t *testing.T) {
	styleCache := new(AuthStyleCache)
	const clientID = "client-id"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token": "ACCESS_TOKEN", "token_type": "bearer"}`)
	}))
	defer ts.Close()

	_, err := RetrieveToken(context.Background(), clientID, "", ts.URL, url.Values{}, AuthStyleUnknown, styleCache)
	if err != nil {
		t.Errorf("RetrieveToken (with background context) = %v; want no error", err)
	}

	retrieved := make(chan struct{})
	cancellingts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-retrieved
	}))
	defer cancellingts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = RetrieveToken(ctx, clientID, "", cancellingts.URL, url.Values{}, AuthStyleUnknown, styleCache)
	close(retrieved)
	if err == nil {
		t.Errorf("RetrieveToken (with cancelled context) = nil; want error")
	}
}

func TestExpiresInUpperBound(t *testing.T) {
	var e expirationTime
	if err := e.UnmarshalJSON([]byte(fmt.Sprint(int64(math.MaxInt32) + 1))); err != nil {
		t.Fatal(err)
	}
	const want = math.MaxInt32
	if e != want {
		t.Errorf("expiration time = %v; want %v", e, want)
	}
}

func TestAuthStyleCache(t *testing.T) {
	var c LazyAuthStyleCache

	cases := []struct {
		url      string
		clientID string
		style    AuthStyle
	}{
		{
			"https://host1.example.com/token",
			"client_1",
			AuthStyleInHeader,
		}, {
			"https://host2.example.com/token",
			"client_2",
			AuthStyleInParams,
		}, {
			"https://host1.example.com/token",
			"client_3",
			AuthStyleInParams,
		},
	}

	for _, tt := range cases {
		t.Run(tt.clientID, func(t *testing.T) {
			cc := c.Get()
			got, ok := cc.lookupAuthStyle(tt.url, tt.clientID)
			if ok {
				t.Fatalf("unexpected auth style found on first request: %v", got)
			}

			cc.setAuthStyle(tt.url, tt.clientID, tt.style)

			got, ok = cc.lookupAuthStyle(tt.url, tt.clientID)
			if !ok {
				t.Fatalf("auth style not found in cache")
			}

			if got != tt.style {
				t.Fatalf("auth style mismatch, got=%v, want=%v", got, tt.style)
			}
		})
	}
}
