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
	"context"
	"net/url"

	"github.com/gorilla/websocket"
	"shanhu.io/aries/creds"
	"shanhu.io/misc/errcode"
	"shanhu.io/misc/httputil"
)

// DialOption provides addition option for dialing.
type DialOption struct {
	Dialer        *websocket.Dialer
	TokenSource   httputil.TokenSource
	Login         *creds.Login
	GuestToken    string
	TunnelOptions *Options
}

// Dial connects to fabrics server, establishes a tunnel and returns an
// endpoint.
func Dial(
	ctx context.Context, addr *url.URL, opt *DialOption,
) (*Endpoint, error) {
	if opt == nil {
		opt = &DialOption{}
	}

	var token string
	if opt.TokenSource != nil {
		t, err := opt.TokenSource.Token(ctx)
		if err != nil {
			return nil, errcode.Annotate(err, "get token from source")
		}
		token = t
	} else if opt.Login != nil {
		t, err := opt.Login.Token()
		if err != nil {
			return nil, errcode.Annotate(err, "login for token")
		}
		token = t
	}

	dialer := &websocketDialer{
		url:        addr,
		token:      token,
		guestToken: opt.GuestToken,
		dialer:     opt.Dialer,
	}
	tunnOpt := opt.TunnelOptions
	if tunnOpt == nil {
		tunnOpt = &Options{}
	}
	conn, err := dialer.dial(ctx, tunnOpt)
	if err != nil {
		return nil, err
	}
	return newEndpoint(conn, dialer, tunnOpt), nil
}
