// Copyright (C) 2021  Shanhu Tech Inc.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package sniproxy

import (
	"testing"

	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"

	"shanhu.io/aries"
	"shanhu.io/aries/https/httpstest"
	"shanhu.io/misc/httputil"
)

func dialURL(addr net.Addr) *url.URL {
	return &url.URL{
		Scheme: "ws",
		Host:   addr.String(),
	}
}

func TestServer_close(t *testing.T) {
	clientChan := make(chan *endpointClient, 1)

	config := &ServerConfig{}
	s := NewServer(config)
	s.setEndpointCallback(func(u string, c *endpointClient) {
		clientChan <- c
	})
	ts := httptest.NewServer(aries.Func(func(c *aries.C) error {
		c.User = "tester"
		return s.ServeBack(c)
	}))
	defer ts.Close()

	u := dialURL(ts.Listener.Addr())
	ep, err := Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatal("dial:", err)
	}
	if err := ep.Close(); err != nil {
		t.Error("close endpoint:", err)
	}
}

func TestServer_endpoint(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	clientChan := make(chan *endpointClient, 1)

	config := &ServerConfig{}
	s := NewServer(config)
	s.setEndpointCallback(func(u string, c *endpointClient) {
		clientChan <- c
	})
	ts := httptest.NewServer(aries.Func(func(c *aries.C) error {
		c.User = "tester"
		return s.ServeBack(c)
	}))
	defer ts.Close()

	u := dialURL(ts.Listener.Addr())

	ep, err := Dial(context.Background(), u, nil)
	if err != nil {
		t.Fatal("dial: ", err)
	}
	defer func() {
		if err := ep.Close(); err != nil {
			log.Println("endpoint close:", err)
		}
	}()

	client := <-clientChan

	t.Run("hello", func(t *testing.T) {
		resp, err := client.Hello(context.Background(), "hello")
		if err != nil {
			t.Fatal("hello:", err)
		}
		if want := "hello"; resp != want {
			t.Errorf("want response %q, got %q", want, resp)
		}

		resp, err = client.Hello(context.Background(), "hello again")
		if err != nil {
			t.Fatal("hello again:", err)
		}
		if want := "hello again"; resp != want {
			t.Errorf("want response %q, got %q", want, resp)
		}
	})

	t.Run("http", func(t *testing.T) {
		host := &http.Server{
			Handler: aries.StringFunc("welcome"),
		}
		go host.Serve(ep) // serve on the endpoint

		dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
			return client.Dial(ctx, "")
		}

		tr := &http.Transport{DialContext: dial}
		hc := &http.Client{Transport: tr}
		resp, err := httputil.GetString(hc, "http://x/")
		if err != nil {
			t.Fatal("http get:", err)
		}
		if resp != "welcome" {
			t.Errorf("want `welcome`, got %q", resp)
		}
	})
}

func TestServer_proxy(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	localhost := &net.TCPAddr{IP: net.IPv6loopback}
	lis, err := net.ListenTCP("tcp", localhost)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	config := &ServerConfig{
		Lookup: func(domain string) (string, error) {
			if domain == "site1.com" {
				return "/site1", nil
			}
			if domain == "site2.com" {
				return "/site2", nil
			}
			return "", fmt.Errorf("bad domain: %q", domain)
		},
	}

	s := NewServer(config)
	ts := httptest.NewServer(aries.Func(func(c *aries.C) error {
		c.User = c.Path
		return s.ServeBack(c)
	}))
	defer ts.Close()

	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(ctx context.Context) {
		defer wg.Done()
		if err := s.ServeFront(ctx, lis); err != nil {
			if !IsClosedConnError(err) {
				log.Println("serve front listener:", err)
			}
		}
	}(ctx)

	makeURL := func(s string) *url.URL {
		return &url.URL{
			Scheme: "ws",
			Host:   ts.Listener.Addr().String(),
			Path:   s,
		}
	}

	ep1, err := Dial(ctx, makeURL("/site1"), nil)
	if err != nil {
		t.Fatal("dial endpoint1: ", err)
	}
	defer ep1.Close()

	ep2, err := Dial(ctx, makeURL("/site2"), nil)
	if err != nil {
		t.Fatal("dial endpoint2: ", err)
	}
	defer ep2.Close()

	// Start two sites.
	tlsConfigs, err := httpstest.NewTLSConfigs([]string{
		"site1.com",
		"site2.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	site1 := &httptest.Server{
		TLS:      tlsConfigs.Server,
		Listener: ep1,
		Config: &http.Server{
			Handler: aries.StringFunc("site1"),
		},
	}
	site1.StartTLS()
	defer site1.Close()

	site2 := &httptest.Server{
		TLS:      tlsConfigs.Server,
		Listener: ep2,
		Config: &http.Server{
			Handler: aries.StringFunc("site2"),
		},
	}
	site2.StartTLS()
	defer site2.Close()

	// Now, need to dial to our server, which is listening
	// on lis.

	client := &http.Client{
		Transport: tlsConfigs.Sink(lis.Addr().String()),
	}

	got, err := httputil.GetString(client, "https://site1.com")
	if err != nil {
		t.Fatal("get:", err)
	}
	if got != "site1" {
		t.Errorf("got %q, want site", got)
	}

	got, err = httputil.GetString(client, "https://site2.com")
	if err != nil {
		t.Fatal("get site2:", err)
	}
	if got != "site2" {
		t.Errorf("got %q, want site2", got)
	}
}

func TestServer_kick(t *testing.T) {
	var wg sync.WaitGroup
	defer wg.Wait()

	localhost := &net.TCPAddr{IP: net.IPv6loopback}
	lis, err := net.ListenTCP("tcp", localhost)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	config := &ServerConfig{
		Lookup: func(domain string) (string, error) {
			if domain == "site.com" {
				return "tester", nil
			}
			return "", fmt.Errorf("bad domain: %q", domain)
		},
	}
	s := NewServer(config)
	ts := httptest.NewServer(aries.Func(func(c *aries.C) error {
		c.User = "tester"
		return s.ServeBack(c)
	}))
	defer ts.Close()

	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(ctx context.Context) {
		defer wg.Done()
		if err := s.ServeFront(ctx, lis); err != nil {
			log.Println("serve front listener:", err)
		}
	}(ctx)

	u := dialURL(ts.Listener.Addr())

	ep1, err := Dial(ctx, u, nil)
	if err != nil {
		t.Fatal("dial: ", err)
	}
	defer func() {
		if err := ep1.Close(); err != nil {
			log.Println("endpoint1 close:", err)
		}
	}()

	tlsConfigs, err := httpstest.NewTLSConfigs([]string{"site.com"})
	if err != nil {
		t.Fatal(err)
	}

	site1 := &httptest.Server{
		TLS:      tlsConfigs.Server,
		Listener: ep1,
		Config:   &http.Server{Handler: aries.StringFunc("site1")},
	}
	site1.StartTLS()
	defer site1.Close()

	client := &http.Client{
		Transport: tlsConfigs.Sink(lis.Addr().String()),
	}

	got1, err := httputil.GetString(client, "https://site.com")
	if err != nil {
		t.Fatal("get:", err)
	}
	if got1 != "site1" {
		t.Errorf("got %q, want site", got1)
	}

	// Perform the kick.
	ep2, err := Dial(ctx, u, nil)
	if err != nil {
		t.Fatal("kick: ", err)
	}

	site2 := &httptest.Server{
		TLS:      tlsConfigs.Server,
		Listener: ep2,
		Config:   &http.Server{Handler: aries.StringFunc("site2")},
	}
	site2.StartTLS()
	defer site2.Close()

	log.Println("another try after kicking")

	got2, err := httputil.GetString(client, "https://site.com")
	if err != nil {
		t.Fatal("get:", err)
	}
	if got2 != "site2" {
		t.Errorf("got %q, want site", got2)
	}

	if err := ep2.Close(); err != nil {
		t.Error("close endpoint2:", err)
	}
}
